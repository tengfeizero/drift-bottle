package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var (
	// ErrUserNotFound 用户不存在
	ErrUserNotFound = errors.New("用户不存在")
	// ErrIDConflict 生成的 ID 已存在
	ErrIDConflict = errors.New("ID 已存在")
)

// User 账户
type User struct {
	ID           string
	Nickname     string
	Avatar       string
	AvatarPreset int
	Gender       int
	Age          int
	PasswordHash string
	PasswordSalt string
	CreatedAt    int64
	LastLoginAt  int64
	TokenVersion int
}

const userColumns = `id, nickname, avatar, avatar_preset, gender, age, password_hash, password_salt, created_at, last_login_at, token_version`

func scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Nickname, &u.Avatar, &u.AvatarPreset, &u.Gender, &u.Age,
		&u.PasswordHash, &u.PasswordSalt, &u.CreatedAt, &u.LastLoginAt, &u.TokenVersion)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateUser 创建账户；ID 重复时返回 ErrIDConflict。
func (s *Store) CreateUser(ctx context.Context, u *User) error {
	const q = `INSERT INTO users
		(id, nickname, avatar, avatar_preset, gender, age, password_hash, password_salt, created_at, last_login_at, token_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q, u.ID, u.Nickname, u.Avatar, u.AvatarPreset, u.Gender, u.Age,
		u.PasswordHash, u.PasswordSalt, u.CreatedAt, u.LastLoginAt, u.TokenVersion)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return ErrIDConflict
	}
	return err
}

// GetUserByID 按 ID 查询账户；不存在时返回 ErrUserNotFound。
func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return u, err
}

// UpdateLastLogin 更新最近登录时间（清理任务的活跃判据）。
func (s *Store) UpdateLastLogin(ctx context.Context, id string, at int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET last_login_at = ? WHERE id = ?`, at, id)
	return err
}

// IncrementTokenVersion 令牌版本号自增，使该用户已签发的全部凭证立即失效。
func (s *Store) IncrementTokenVersion(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET token_version = token_version + 1 WHERE id = ?`, id)
	return err
}

// ListInactiveUserIDs 返回最近登录时间早于 before 的用户 ID（清理任务的候选集合）。
func (s *Store) ListInactiveUserIDs(ctx context.Context, before int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM users WHERE last_login_at < ?`, before)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteUser 物理删除账户及其全部数据：作为投放者或捞取者的瓶子（消息与已读记录由外键级联删除）、
// 每日配额用量与账户本身，全部在同一事务内完成。
// 返回被删除用户的头像相对路径（可能为空）与相关附件文件的相对路径，供调用方清理磁盘文件。
func (s *Store) DeleteUser(ctx context.Context, id string) (string, []string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var avatar string
	err = tx.QueryRowContext(ctx, `SELECT avatar FROM users WHERE id = ?`, id).Scan(&avatar)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, ErrUserNotFound
	}
	if err != nil {
		return "", nil, err
	}

	paths, err := deleteUserAttachmentsTx(ctx, tx, id)
	if err != nil {
		return "", nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM bottles WHERE owner_id = ? OR holder_id = ?`, id, id); err != nil {
		return "", nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM daily_usage WHERE user_id = ?`, id); err != nil {
		return "", nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id); err != nil {
		return "", nil, err
	}
	if err := tx.Commit(); err != nil {
		return "", nil, err
	}
	return avatar, paths, nil
}

// UpdateProfile 更新昵称、性别与年龄。
func (s *Store) UpdateProfile(ctx context.Context, id, nickname string, gender, age int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET nickname = ?, gender = ?, age = ? WHERE id = ?`,
		nickname, gender, age, id)
	return err
}

// UpdateAvatar 更新头像相对路径（空字符串表示使用系统头像），同时清除 avatar_preset 以保证头像来源唯一。
func (s *Store) UpdateAvatar(ctx context.Context, id, avatar string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET avatar = ?, avatar_preset = 0 WHERE id = ?`, avatar, id)
	return err
}

// UpdateAvatarPreset 设定用户选中的系统头像序号（preset ∈ [1,12]），同时清空上传头像路径。
// preset = 0 表示"恢复默认"——清除 preset，avatar 也保持空串，前端将按 uid 哈希分配默认生肖。
func (s *Store) UpdateAvatarPreset(ctx context.Context, id string, preset int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET avatar = '', avatar_preset = ? WHERE id = ?`, preset, id)
	return err
}

// UpdatePassword 更新密码哈希与盐。
func (s *Store) UpdatePassword(ctx context.Context, id, hash, salt string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, password_salt = ? WHERE id = ?`,
		hash, salt, id)
	return err
}
