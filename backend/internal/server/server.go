package server

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"driftbottle/internal/config"
	"driftbottle/internal/store"
	"driftbottle/web"
)

// Server HTTP 服务
type Server struct {
	cfg   config.Config
	store *store.Store
	ws    *wsHub
	// onlineCheck 判断用户当前是否在线（是否存在 WebSocket 活跃连接）
	onlineCheck func(userID string) bool
}

// New 创建服务实例：打开数据库并执行迁移
func New(cfg config.Config) (*Server, error) {
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	if err := st.Migrate(); err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, uploadsDirName, attachmentsDirName), 0o755); err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}
	registerAttachmentMIMETypes()
	s := &Server{cfg: cfg, store: st, ws: newWSHub()}
	s.onlineCheck = s.ws.isOnline
	return s, nil
}

// Start 启动服务，同一端口提供静态页面、API 与（后续的）WebSocket
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API 路由
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/register", s.handleRegister)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.requireAuth(s.handleLogout))
	mux.HandleFunc("GET /api/me", s.requireAuth(s.handleMe))
	mux.HandleFunc("PUT /api/me/profile", s.requireAuth(s.handleUpdateProfile))
	mux.HandleFunc("POST /api/me/avatar", s.requireAuth(s.handleUploadAvatar))
	mux.HandleFunc("PUT /api/me/avatar-preset", s.requireAuth(s.handleAvatarPreset))
	mux.HandleFunc("PUT /api/me/password", s.requireAuth(s.handleChangePassword))
	mux.HandleFunc("POST /api/bottles", s.requireAuth(s.handleCreateBottle))
	mux.HandleFunc("POST /api/attachments", s.requireAuth(s.handleUploadAttachment))
	mux.HandleFunc("POST /api/bottles/pick", s.requireAuth(s.handlePickBottle))
	mux.HandleFunc("POST /api/bottles/{id}/return", s.requireAuth(s.handleReturnBottle))
	mux.HandleFunc("GET /api/bottles", s.requireAuth(s.handleListBottles))
	mux.HandleFunc("GET /api/bottles/{id}", s.requireAuth(s.handleBottleDetail))
	mux.HandleFunc("DELETE /api/bottles/{id}", s.requireAuth(s.handleDeleteBottle))
	mux.HandleFunc("POST /api/bottles/{id}/messages", s.requireAuth(s.handleSendMessage))
	mux.HandleFunc("GET /api/quotas", s.requireAuth(s.handleQuotas))

	// WebSocket（实时推送）
	mux.HandleFunc("GET /ws", s.handleWS)

	// 上传文件（头像）静态访问
	mux.Handle("GET /uploads/", noSniff(http.StripPrefix("/uploads/",
		http.FileServer(http.Dir(filepath.Join(s.cfg.DataDir, uploadsDirName))))))

	// 前端静态资源（内嵌产物）
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return err
	}
	mux.Handle("/", spaHandler(dist))

	addr := ":" + s.cfg.Port
	log.Printf("漂流瓶服务启动: http://localhost%s (数据目录: %s)", addr, s.cfg.DataDir)

	s.startCleanup()
	return http.ListenAndServe(addr, mux)
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]string{"status": "up"})
}

// noSniff 为上传内容附加 X-Content-Type-Options，禁止浏览器按内容嗅探类型
func noSniff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// spaHandler 静态资源服务；未命中文件时回退到 index.html，以支持前端路由
func spaHandler(dist fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(dist, p); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
