package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Store 数据访问层，封装 SQLite 连接
type Store struct {
	db *sql.DB
}

// Open 打开（或创建）SQLite 数据库，启用 WAL、外键与忙等待
func Open(dataDir string) (*Store, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)",
		filepath.ToSlash(filepath.Join(dataDir, "app.db")),
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite 实际只支持单写入者：限制连接数以避免并发写导致 database is locked
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close 关闭数据库连接
func (s *Store) Close() error {
	return s.db.Close()
}

// Migrate 执行建表与建索引（幂等）
func (s *Store) Migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range schema {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("执行迁移失败: %w", err)
		}
	}

	// 增量迁移：为旧库补上 avatar_preset 列（SQLite ALTER TABLE 不支持 IF NOT EXISTS，
	// 重复执行时会返回 duplicate column 错误，忽略即可）。
	const alterAvatarPreset = `ALTER TABLE users ADD COLUMN avatar_preset INTEGER NOT NULL DEFAULT 0`
	if _, err := tx.Exec(alterAvatarPreset); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("执行增量迁移失败: %w", err)
		}
	}

	return tx.Commit()
}
