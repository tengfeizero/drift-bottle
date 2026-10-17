package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// 附件类型
const (
	AttachmentKindImage = "image"
	AttachmentKindAudio = "audio"
	AttachmentKindVideo = "video"
)

// ErrAttachmentInvalid 附件不存在、已被使用或不属于当前用户
var ErrAttachmentInvalid = errors.New("附件无效")

// Attachment 媒体附件；BottleID 与 MessageID 同时无效表示尚未绑定。
type Attachment struct {
	ID        int64
	OwnerID   string
	BottleID  sql.NullInt64
	MessageID sql.NullInt64
	Kind      string
	Mime      string
	Size      int64
	OrigName  string
	Path      string
	CreatedAt int64
}

const attachmentColumns = `id, owner_id, bottle_id, message_id, kind, mime, size, orig_name, path, created_at`

func scanAttachment(row *sql.Row) (*Attachment, error) {
	var a Attachment
	err := row.Scan(&a.ID, &a.OwnerID, &a.BottleID, &a.MessageID,
		&a.Kind, &a.Mime, &a.Size, &a.OrigName, &a.Path, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// CreateAttachment 记录一个已上传、尚未绑定的附件。
func (s *Store) CreateAttachment(ctx context.Context, a *Attachment) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO attachments (owner_id, bottle_id, message_id, kind, mime, size, orig_name, path, created_at)
		 VALUES (?, NULL, NULL, ?, ?, ?, ?, ?, ?)`,
		a.OwnerID, a.Kind, a.Mime, a.Size, a.OrigName, a.Path, a.CreatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	a.ID = id
	return nil
}

// GetAttachmentByID 按 ID 查询附件；不存在返回 ErrAttachmentInvalid。
func (s *Store) GetAttachmentByID(ctx context.Context, id int64) (*Attachment, error) {
	a, err := scanAttachment(s.db.QueryRowContext(ctx,
		`SELECT `+attachmentColumns+` FROM attachments WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAttachmentInvalid
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// AttachmentByBottleID 返回瓶子投放内容的附件；无则返回 (nil, nil)。
func (s *Store) AttachmentByBottleID(ctx context.Context, bottleID int64) (*Attachment, error) {
	a, err := scanAttachment(s.db.QueryRowContext(ctx,
		`SELECT `+attachmentColumns+` FROM attachments WHERE bottle_id = ? LIMIT 1`, bottleID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// AttachmentByMessageID 返回某条消息的附件；无则返回 (nil, nil)。
func (s *Store) AttachmentByMessageID(ctx context.Context, messageID int64) (*Attachment, error) {
	a, err := scanAttachment(s.db.QueryRowContext(ctx,
		`SELECT `+attachmentColumns+` FROM attachments WHERE message_id = ? LIMIT 1`, messageID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// AttachmentsByBottleIDs 批量返回各瓶子投放内容的附件，键为瓶子 ID。
func (s *Store) AttachmentsByBottleIDs(ctx context.Context, bottleIDs []int64) (map[int64]*Attachment, error) {
	return s.attachmentsByIDColumn(ctx, "bottle_id", bottleIDs)
}

// AttachmentsByMessageIDs 批量返回各消息的附件，键为消息 ID。
func (s *Store) AttachmentsByMessageIDs(ctx context.Context, messageIDs []int64) (map[int64]*Attachment, error) {
	return s.attachmentsByIDColumn(ctx, "message_id", messageIDs)
}

func (s *Store) attachmentsByIDColumn(ctx context.Context, column string, ids []int64) (map[int64]*Attachment, error) {
	result := make(map[int64]*Attachment, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	query := `SELECT ` + attachmentColumns + ` FROM attachments
		WHERE ` + column + ` IN (?` + strings.Repeat(", ?", len(ids)-1) + `)`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.OwnerID, &a.BottleID, &a.MessageID,
			&a.Kind, &a.Mime, &a.Size, &a.OrigName, &a.Path, &a.CreatedAt); err != nil {
			return nil, err
		}
		if a.BottleID.Valid {
			result[a.BottleID.Int64] = &a
		} else if a.MessageID.Valid {
			result[a.MessageID.Int64] = &a
		}
	}
	return result, rows.Err()
}

// bindBottleAttachmentTx 在同一事务内把未绑定附件挂到瓶子上；不可用返回 ErrAttachmentInvalid。
func bindBottleAttachmentTx(ctx context.Context, tx *sql.Tx, attachmentID int64, ownerID string, bottleID int64) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE attachments SET bottle_id = ?
		 WHERE id = ? AND owner_id = ? AND bottle_id IS NULL AND message_id IS NULL`,
		bottleID, attachmentID, ownerID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrAttachmentInvalid
	}
	return nil
}

// bindMessageAttachmentTx 在同一事务内把未绑定附件挂到消息上；不可用返回 ErrAttachmentInvalid。
func bindMessageAttachmentTx(ctx context.Context, tx *sql.Tx, attachmentID int64, ownerID string, messageID int64) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE attachments SET message_id = ?
		 WHERE id = ? AND owner_id = ? AND bottle_id IS NULL AND message_id IS NULL`,
		messageID, attachmentID, ownerID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrAttachmentInvalid
	}
	return nil
}

// PurgeOrphanAttachments 删除超过保留期仍未绑定的附件记录，返回待清理的文件相对路径。
func (s *Store) PurgeOrphanAttachments(ctx context.Context, before int64) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	paths, err := deleteAttachmentsTx(ctx, tx,
		`bottle_id IS NULL AND message_id IS NULL AND created_at < ?`, before)
	if err != nil {
		return nil, err
	}
	return paths, tx.Commit()
}

// deleteAttachmentsTx 在事务内删除满足条件的附件记录，返回其文件相对路径。
func deleteAttachmentsTx(ctx context.Context, tx *sql.Tx, condition string, args ...any) ([]string, error) {
	paths, err := collectPathsTx(ctx, tx, `SELECT path FROM attachments WHERE `+condition, args...)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM attachments WHERE `+condition, args...); err != nil {
		return nil, err
	}
	return paths, nil
}

// deleteBottleAttachmentsTx 在事务内删除某瓶子及其消息的全部附件记录，返回文件相对路径。
// 必须在删除瓶子（消息由外键级联删除）之前调用。
func deleteBottleAttachmentsTx(ctx context.Context, tx *sql.Tx, bottleID int64) ([]string, error) {
	const cond = `(bottle_id = ? OR message_id IN (SELECT id FROM messages WHERE bottle_id = ?))`
	return deleteAttachmentsTx(ctx, tx, cond, bottleID, bottleID)
}

// deleteUserAttachmentsTx 在事务内删除某用户相关的全部附件记录（其上传待绑定的、作为投放者或
// 捞取者参与的瓶子及其消息），返回文件相对路径。必须在删除瓶子与账户之前调用。
func deleteUserAttachmentsTx(ctx context.Context, tx *sql.Tx, userID string) ([]string, error) {
	const cond = `(owner_id = ? AND bottle_id IS NULL AND message_id IS NULL)
		OR bottle_id IN (SELECT id FROM bottles WHERE owner_id = ? OR holder_id = ?)
		OR message_id IN (SELECT id FROM messages
			WHERE bottle_id IN (SELECT id FROM bottles WHERE owner_id = ? OR holder_id = ?))`
	return deleteAttachmentsTx(ctx, tx, cond, userID, userID, userID, userID, userID)
}

// deleteExpiredMessageAttachmentsTx 在事务内删除已过期消息的附件记录，返回文件相对路径。
// 必须在删除消息之前调用。
func deleteExpiredMessageAttachmentsTx(ctx context.Context, tx *sql.Tx, now int64) ([]string, error) {
	return deleteAttachmentsTx(ctx, tx,
		`message_id IN (SELECT id FROM messages WHERE expire_at <= ?)`, now)
}

func collectPathsTx(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
