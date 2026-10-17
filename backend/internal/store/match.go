package store

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNoMatch 海里没有符合条件的瓶子
var ErrNoMatch = errors.New("无匹配瓶子")

// rowQuerier 可执行单行查询的对象（*sql.DB 与 *sql.Tx 均满足）
type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// matchBottleQuery 随机取一个满足匹配条件且处于 SEA 状态的瓶子。
// 匹配条件：捞取者年龄 ∈ 瓶子目标年龄范围，且瓶子目标性别为不限或等于捞取者性别；排除自己投放的瓶子。
const matchBottleQuery = `
SELECT id, owner_id, holder_id, content, target_gender, target_age_min,
       target_age_max, state, created_at, last_message_at
FROM bottles
WHERE state = ?
  AND ? BETWEEN target_age_min AND target_age_max
  AND (target_gender = ? OR target_gender = ?)
  AND owner_id <> ?
ORDER BY RANDOM()
LIMIT 1`

func pickRandomBottle(ctx context.Context, q rowQuerier, age, gender int, excludeOwnerID string) (*Bottle, error) {
	var b Bottle
	err := q.QueryRowContext(ctx, matchBottleQuery,
		BottleStateSea, age, GenderUnset, gender, excludeOwnerID,
	).Scan(
		&b.ID, &b.OwnerID, &b.HolderID, &b.Content, &b.TargetGender,
		&b.TargetAgeMin, &b.TargetAgeMax, &b.State, &b.CreatedAt, &b.LastMessageAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// PickBottleWithQuota 在同一事务内扣减当日捞取配额、匹配一个瓶子并置为 HELD。
// 配额用尽返回 ErrQuotaExceeded；无匹配返回 ErrNoMatch（不消耗配额）。
func (s *Store) PickBottleWithQuota(ctx context.Context, age, gender int, userID, date string, limit int) (*Bottle, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var used int
	err = tx.QueryRowContext(ctx,
		`SELECT pick_count FROM daily_usage WHERE user_id = ? AND usage_date = ?`,
		userID, date).Scan(&used)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if used >= limit {
		return nil, ErrQuotaExceeded
	}

	b, err := pickRandomBottle(ctx, tx, age, gender, userID)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, ErrNoMatch
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE bottles SET holder_id = ?, state = ? WHERE id = ? AND state = ?`,
		userID, BottleStateHeld, b.ID, BottleStateSea); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO daily_usage (user_id, usage_date, throw_count, pick_count) VALUES (?, ?, 0, 0)
		 ON CONFLICT(user_id, usage_date) DO NOTHING`,
		userID, date); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE daily_usage SET pick_count = pick_count + 1 WHERE user_id = ? AND usage_date = ?`,
		userID, date); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	b.HolderID = sql.NullString{String: userID, Valid: true}
	b.State = BottleStateHeld
	return b, nil
}
