package server

import (
	"errors"
	"net/http"
	"time"

	"driftbottle/internal/auth"
	"driftbottle/internal/store"
)

// idGenRetries ID 冲突时的重试次数
const idGenRetries = 5

type registerRequest struct {
	Password string `json:"password"`
}

type loginRequest struct {
	ID       string `json:"id"`
	Password string `json:"password"`
}

type authResponse struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Token    string `json:"token"`
}

// handleRegister 注册：仅需密码，返回系统生成的 ID 与登录凭证。
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, CodePasswordInvalid, "密码需为 6-32 位且至少包含字母与数字")
		return
	}

	hash, salt, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodePasswordInvalid, "密码处理失败")
		return
	}

	now := time.Now().UnixMilli()
	var user *store.User
	for i := 0; i < idGenRetries; i++ {
		id, err := auth.GenerateID()
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeIDGenerationFailed, "生成 ID 失败")
			return
		}
		u := &store.User{
			ID:           id,
			Nickname:     "用户" + id[len(id)-4:],
			Gender:       store.GenderUnset,
			PasswordHash: hash,
			PasswordSalt: salt,
			CreatedAt:    now,
			LastLoginAt:  now,
		}
		if err := s.store.CreateUser(r.Context(), u); err != nil {
			if errors.Is(err, store.ErrIDConflict) {
				continue // 极低概率冲突，重新生成
			}
			writeError(w, http.StatusInternalServerError, CodeIDGenerationFailed, "创建账户失败")
			return
		}
		user = u
		break
	}
	if user == nil {
		writeError(w, http.StatusInternalServerError, CodeIDGenerationFailed, "生成 ID 失败")
		return
	}

	token, err := auth.SignToken(s.cfg.JWTSecret, user.ID, user.TokenVersion, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeUnauthorized, "签发凭证失败")
		return
	}
	writeOK(w, authResponse{ID: user.ID, Nickname: user.Nickname, Token: token})
}

// handleLogin 登录：ID + 密码，成功则更新最近登录时间并签发凭证。
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := s.store.GetUserByID(r.Context(), req.ID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, CodeIDOrPasswordWrong, "ID 或密码错误")
		return
	}
	if !auth.VerifyPassword(req.Password, u.PasswordHash, u.PasswordSalt) {
		writeError(w, http.StatusUnauthorized, CodeIDOrPasswordWrong, "ID 或密码错误")
		return
	}

	now := time.Now()
	if err := s.store.UpdateLastLogin(r.Context(), u.ID, now.UnixMilli()); err != nil {
		writeError(w, http.StatusInternalServerError, CodeIDOrPasswordWrong, "登录失败")
		return
	}

	token, err := auth.SignToken(s.cfg.JWTSecret, u.ID, u.TokenVersion, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeUnauthorized, "签发凭证失败")
		return
	}
	writeOK(w, authResponse{ID: u.ID, Nickname: u.Nickname, Token: token})
}

// handleLogout 退出登录：令牌版本号自增，使已签发凭证立即失效。
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if err := s.store.IncrementTokenVersion(r.Context(), u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, CodeUnauthorized, "退出失败")
		return
	}
	writeOK(w, nil)
}
