package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// MessageTTL 单条消息有效期
const MessageTTL = 24 * time.Hour

// ErrNotParticipant 当前用户不是该瓶子的投放者或捞取者
var ErrNotParticipant = errors.New("无权操作该瓶子")

// Message 会话消息
type Message struct {
	ID        int64
	BottleID  int64
	SenderID  string
	Content   string
	CreatedAt int64
	ExpireAt  int64
}

// ListMessages 返回瓶子内未过期的消息，按时间升序。
func (s *Store) ListMessages(ctx context.Context, bottleID, now int64) ([]*Message, error) {
	const query = `SELECT id, bottle_id, sender_id, content, created_at, expire_at
		FROM messages WHERE bottle_id = ? AND expire_at > ? ORDER BY created_at ASC`
	rows, err := s.db.QueryContext(ctx, query, bottleID, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []*Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.BottleID, &m.SenderID, &m.Content, &m.CreatedAt, &m.ExpireAt); err != nil {
			return nil, err
		}
		list = append(list, &m)
	}
	return list, rows.Err()
}

// LastVisibleMessage 返回瓶子内最近一条未过期消息；无则返回 (nil, nil)。
func (s *Store) LastVisibleMessage(ctx context.Context, bottleID, now int64) (*Message, error) {
	const query = `SELECT id, bottle_id, sender_id, content, created_at, expire_at
		FROM messages WHERE bottle_id = ? AND expire_at > ? ORDER BY created_at DESC LIMIT 1`
	var m Message
	err := s.db.QueryRowContext(ctx, query, bottleID, now).Scan(
		&m.ID, &m.BottleID, &m.SenderID, &m.Content, &m.CreatedAt, &m.ExpireAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// HasUnread 判断是否存在来自对方、晚于本人最近已读时间且未过期的消息。
func (s *Store) HasUnread(ctx context.Context, bottleID int64, userID string, now int64) (bool, error) {
	const query = `SELECT EXISTS(
		SELECT 1 FROM messages m
		WHERE m.bottle_id = ?
		  AND m.sender_id <> ?
		  AND m.expire_at > ?
		  AND m.created_at > COALESCE(
		      (SELECT r.last_read_at FROM bottle_reads r WHERE r.bottle_id = ? AND r.user_id = ?), 0))`
	var unread bool
	err := s.db.QueryRowContext(ctx, query, bottleID, userID, now, bottleID, userID).Scan(&unread)
	return unread, err
}

// MarkRead 记录用户对瓶子的最近已读时间。
func (s *Store) MarkRead(ctx context.Context, bottleID int64, userID string, at int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bottle_reads (bottle_id, user_id, last_read_at) VALUES (?, ?, ?)
		 ON CONFLICT(bottle_id, user_id) DO UPDATE SET last_read_at = excluded.last_read_at`,
		bottleID, userID, at)
	return err
}

// PurgeExpiredMessages 物理删除所有已过期消息（expire_at <= now），返回删除条数与待清理的附件文件相对路径。
func (s *Store) PurgeExpiredMessages(ctx context.Context, now int64) (int64, []string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	paths, err := deleteExpiredMessageAttachmentsTx(ctx, tx, now)
	if err != nil {
		return 0, nil, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE expire_at <= ?`, now)
	if err != nil {
		return 0, nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(); err != nil {
		return 0, nil, err
	}
	return n, paths, nil
}

// SendMessage 发送消息：仅投放者或捞取者可发送；HELD 状态首次回复后转为 CHATTING。
// attachmentID 非 0 时把该未绑定附件挂到新消息上（不可用返回 ErrAttachmentInvalid，整条消息回滚）。
func (s *Store) SendMessage(ctx context.Context, bottleID int64, senderID, content string, attachmentID int64, now time.Time) (*Message, error) {
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

	if b.State == BottleStateSea || (b.OwnerID != senderID && (!b.HolderID.Valid || b.HolderID.String != senderID)) {
		return nil, ErrNotParticipant
	}

	nowMs := now.UnixMilli()
	expireAt := now.Add(MessageTTL).UnixMilli()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO messages (bottle_id, sender_id, content, created_at, expire_at) VALUES (?, ?, ?, ?, ?)`,
		bottleID, senderID, content, nowMs, expireAt)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	if attachmentID != 0 {
		if err := bindMessageAttachmentTx(ctx, tx, attachmentID, senderID, id); err != nil {
			return nil, err
		}
	}

	nextState := b.State
	if nextState == BottleStateHeld {
		nextState = BottleStateChatting
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE bottles SET last_message_at = ?, state = ? WHERE id = ?`,
		nowMs, nextState, bottleID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &Message{ID: id, BottleID: bottleID, SenderID: senderID, Content: content, CreatedAt: nowMs, ExpireAt: expireAt}, nil
}
