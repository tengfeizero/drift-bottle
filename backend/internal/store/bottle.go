package store

import (
	"context"
	"database/sql"
	"errors"
)

// DateFormat 每日配额的日期格式（Asia/Shanghai 自然日）
const DateFormat = "2006-01-02"

// ErrQuotaExceeded 当日配额已用完
var ErrQuotaExceeded = errors.New("配额已用完")

// ErrBottleNotFound 瓶子不存在或当前用户无权操作
var ErrBottleNotFound = errors.New("瓶子不存在")

// Quota 每日配额用量
type Quota struct {
	Date  string // 日期 YYYY-MM-DD（Asia/Shanghai）
	Throw int    // 当日投放次数
	Pick  int    // 当日捞取次数
}

const bottleColumns = `id, owner_id, holder_id, content, target_gender, target_age_min, target_age_max, state, created_at, last_message_at`

func scanBottle(row *sql.Row) (*Bottle, error) {
	var b Bottle
	err := row.Scan(&b.ID, &b.OwnerID, &b.HolderID, &b.Content, &b.TargetGender,
		&b.TargetAgeMin, &b.TargetAgeMax, &b.State, &b.CreatedAt, &b.LastMessageAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// GetBottleByID 按 ID 查询瓶子；不存在返回 ErrBottleNotFound。
func (s *Store) GetBottleByID(ctx context.Context, id int64) (*Bottle, error) {
	b, err := scanBottle(s.db.QueryRowContext(ctx, `SELECT `+bottleColumns+` FROM bottles WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBottleNotFound
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ListBottles 返回用户参与的瓶子，按最近活动时间倒序。
// 可见范围：作为捞取者可见全部持有过的瓶子；作为投放者可见 SEA 与 CHATTING 的瓶子（HELD 期间仅捞取者可见）。
func (s *Store) ListBottles(ctx context.Context, userID string) ([]*Bottle, error) {
	const query = `SELECT ` + bottleColumns + ` FROM bottles
		WHERE holder_id = ?
		   OR (owner_id = ? AND state <> ?)
		ORDER BY COALESCE(last_message_at, created_at) DESC`
	rows, err := s.db.QueryContext(ctx, query, userID, userID, BottleStateHeld)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []*Bottle
	for rows.Next() {
		var b Bottle
		if err := rows.Scan(&b.ID, &b.OwnerID, &b.HolderID, &b.Content, &b.TargetGender,
			&b.TargetAgeMin, &b.TargetAgeMax, &b.State, &b.CreatedAt, &b.LastMessageAt); err != nil {
			return nil, err
		}
		list = append(list, &b)
	}
	return list, rows.Err()
}

// CreateBottleWithQuota 在同一事务内校验并扣减当日投放配额，然后创建瓶子；
// attachmentID 非 0 时把该未绑定附件挂到新瓶子上（不可用返回 ErrAttachmentInvalid，配额一并回滚）。
// 配额用尽返回 ErrQuotaExceeded。
func (s *Store) CreateBottleWithQuota(ctx context.Context, b *Bottle, attachmentID int64, date string, limit int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var used int
	err = tx.QueryRowContext(ctx,
		`SELECT throw_count FROM daily_usage WHERE user_id = ? AND usage_date = ?`,
		b.OwnerID, date).Scan(&used)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if used >= limit {
		return ErrQuotaExceeded
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO daily_usage (user_id, usage_date, throw_count, pick_count) VALUES (?, ?, 0, 0)
		 ON CONFLICT(user_id, usage_date) DO NOTHING`,
		b.OwnerID, date); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE daily_usage SET throw_count = throw_count + 1 WHERE user_id = ? AND usage_date = ?`,
		b.OwnerID, date); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO bottles (owner_id, content, target_gender, target_age_min, target_age_max, state, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b.OwnerID, b.Content, b.TargetGender, b.TargetAgeMin, b.TargetAgeMax, b.State, b.CreatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	b.ID = id
	if attachmentID != 0 {
		if err := bindBottleAttachmentTx(ctx, tx, attachmentID, b.OwnerID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetQuota 读取指定日期的配额用量；无记录时返回次数为 0 的用量。
func (s *Store) GetQuota(ctx context.Context, userID, date string) (*Quota, error) {
	q := &Quota{Date: date}
	err := s.db.QueryRowContext(ctx,
		`SELECT throw_count, pick_count FROM daily_usage WHERE user_id = ? AND usage_date = ?`,
		userID, date).Scan(&q.Throw, &q.Pick)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return q, nil
}

// DeleteBottle 双删：SEA 状态仅投放者可删除；HELD/CHATTING 状态任一方删除即彻底删除。
// 瓶子及其消息、已读记录被物理删除（后者由外键 ON DELETE CASCADE 级联）；
// 返回附件文件的相对路径，供调用方清理磁盘文件。
func (s *Store) DeleteBottle(ctx context.Context, bottleID int64, userID string) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	b, err := scanBottle(tx.QueryRowContext(ctx, `SELECT `+bottleColumns+` FROM bottles WHERE id = ?`, bottleID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBottleNotFound
	}
	if err != nil {
		return nil, err
	}

	allowed := b.OwnerID == userID
	if b.State != BottleStateSea {
		allowed = allowed || (b.HolderID.Valid && b.HolderID.String == userID)
	}
	if !allowed {
		return nil, ErrBottleNotFound
	}

	paths, err := deleteBottleAttachmentsTx(ctx, tx, bottleID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM bottles WHERE id = ?`, bottleID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return paths, nil
}

// ReturnBottle 扔回海里：仅捞取者可将 HELD 状态的瓶子置回 SEA 并清空捞取者。
func (s *Store) ReturnBottle(ctx context.Context, bottleID int64, userID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE bottles SET holder_id = NULL, state = ? WHERE id = ? AND holder_id = ? AND state = ?`,
		BottleStateSea, bottleID, userID, BottleStateHeld)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrBottleNotFound
	}
	return nil
}
