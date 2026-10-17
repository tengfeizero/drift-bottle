package server

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"driftbottle/internal/store"
)

// inactiveThreshold 账户活跃判据：最近登录时间超过该时长视为不活跃（共享规则 R-03）
const inactiveThreshold = 72 * time.Hour

// orphanAttachmentThreshold 已上传但长时间未绑定到任何内容/消息的附件视为孤儿
const orphanAttachmentThreshold = time.Hour

// startCleanup 启动定时清理任务：过期消息物理清理与不活跃账户清理
func (s *Server) startCleanup() {
	go func() {
		ticker := time.NewTicker(s.cfg.CleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.runCleanup()
		}
	}()
}

// runCleanup 执行一轮清理
func (s *Server) runCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.CleanupInterval)
	defer cancel()
	now := time.Now()

	if n, paths, err := s.store.PurgeExpiredMessages(ctx, now.UnixMilli()); err != nil {
		log.Printf("清理过期消息失败: %v", err)
	} else if n > 0 {
		s.removeUploadFiles(paths)
		log.Printf("已清理过期消息 %d 条", n)
	}

	if paths, err := s.store.PurgeOrphanAttachments(ctx, now.Add(-orphanAttachmentThreshold).UnixMilli()); err != nil {
		log.Printf("清理孤儿附件失败: %v", err)
	} else if len(paths) > 0 {
		s.removeUploadFiles(paths)
		log.Printf("已清理未绑定附件 %d 个", len(paths))
	}

	ids, err := s.store.ListInactiveUserIDs(ctx, now.Add(-inactiveThreshold).UnixMilli())
	if err != nil {
		log.Printf("查询待清理账户失败: %v", err)
		return
	}
	for _, id := range ids {
		if s.isOnline(id) {
			continue
		}
		avatar, paths, err := s.store.DeleteUser(ctx, id)
		if err != nil {
			if !errors.Is(err, store.ErrUserNotFound) {
				log.Printf("清理账户 %s 失败: %v", id, err)
			}
			continue
		}
		if avatar != "" {
			if err := os.Remove(filepath.Join(s.cfg.DataDir, filepath.FromSlash(avatar))); err != nil && !os.IsNotExist(err) {
				log.Printf("清理账户 %s 头像失败: %v", id, err)
			}
		}
		s.removeUploadFiles(paths)
		log.Printf("已清理不活跃账户 %s", id)
	}
}

// isOnline 判断用户当前是否存在活跃连接；M8 接入 WebSocket 后由连接管理器提供判断。
func (s *Server) isOnline(userID string) bool {
	if s.onlineCheck == nil {
		return false
	}
	return s.onlineCheck(userID)
}
