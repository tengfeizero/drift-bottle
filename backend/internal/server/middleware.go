package server

import (
	"context"
	"net/http"
	"strings"

	"driftbottle/internal/auth"
	"driftbottle/internal/store"
)

type contextKey int

const userContextKey contextKey = iota

// currentUser 从请求上下文取出已鉴权用户；仅应在 requireAuth 之后调用。
func currentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(userContextKey).(*store.User)
	return u
}

// requireAuth 鉴权中间件：校验 Bearer 令牌，并比对令牌版本号（支持服务端失效）。
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, "未登录或登录已过期")
			return
		}
		claims, err := auth.ParseToken(s.cfg.JWTSecret, token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, "未登录或登录已过期")
			return
		}
		u, err := s.store.GetUserByID(r.Context(), claims.Subject)
		if err != nil || u.TokenVersion != claims.TokenVersion {
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, "未登录或登录已过期")
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, u)
		next(w, r.WithContext(ctx))
	}
}

// bearerToken 从 Authorization 头提取 Bearer 令牌
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}
