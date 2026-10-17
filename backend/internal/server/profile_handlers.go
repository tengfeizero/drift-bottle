package server

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"driftbottle/internal/auth"
	"driftbottle/internal/store"
)

// uploadsDirName 头像上传目录名（相对数据目录）
const uploadsDirName = "uploads"

// maxAvatarSize 头像文件大小上限
const maxAvatarSize = 2 << 20 // 2 MiB

// avatarExtByType 允许的头像类型与扩展名映射
var avatarExtByType = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// uploadsDir 头像上传目录的绝对路径
func (s *Server) uploadsDir() string {
	return filepath.Join(s.cfg.DataDir, uploadsDirName)
}

type profileResponse struct {
	ID           string `json:"id"`
	Nickname     string `json:"nickname"`
	Avatar       string `json:"avatar"`
	AvatarPreset int    `json:"avatar_preset"`
	Gender       int    `json:"gender"`
	Age          int    `json:"age"`
}

func toProfileResponse(u *store.User) profileResponse {
	avatar := ""
	if u.Avatar != "" {
		avatar = "/" + u.Avatar
	}
	return profileResponse{
		ID:           u.ID,
		Nickname:     u.Nickname,
		Avatar:       avatar,
		AvatarPreset: u.AvatarPreset,
		Gender:       u.Gender,
		Age:          u.Age,
	}
}

// handleMe 返回当前登录用户的资料
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeOK(w, toProfileResponse(currentUser(r)))
}

type profileRequest struct {
	Nickname string `json:"nickname"`
	Gender   int    `json:"gender"`
	Age      int    `json:"age"`
}

// handleUpdateProfile 更新昵称、性别与年龄
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req profileRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	nickname := strings.TrimSpace(req.Nickname)
	if n := utf8.RuneCountInString(nickname); n < 1 || n > 20 {
		writeError(w, http.StatusBadRequest, CodeProfileInvalid, "昵称需为 1-20 个字符")
		return
	}
	if req.Gender != store.GenderUnset && req.Gender != store.GenderMale && req.Gender != store.GenderFemale {
		writeError(w, http.StatusBadRequest, CodeProfileInvalid, "性别取值无效")
		return
	}
	if req.Age < 0 || req.Age > 150 {
		writeError(w, http.StatusBadRequest, CodeProfileInvalid, "年龄需为 0-150")
		return
	}

	u := currentUser(r)
	if err := s.store.UpdateProfile(r.Context(), u.ID, nickname, req.Gender, req.Age); err != nil {
		writeError(w, http.StatusInternalServerError, CodeProfileInvalid, "更新资料失败")
		return
	}
	u.Nickname, u.Gender, u.Age = nickname, req.Gender, req.Age
	writeOK(w, toProfileResponse(u))
}

// handleUploadAvatar 上传头像并替换旧头像
func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarSize)
	file, _, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeAvatarInvalid, "请上传不超过 2MB 的头像文件")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil || len(data) == 0 {
		writeError(w, http.StatusBadRequest, CodeAvatarInvalid, "读取头像文件失败")
		return
	}

	ext, ok := avatarExtByType[http.DetectContentType(data)]
	if !ok {
		writeError(w, http.StatusBadRequest, CodeAvatarInvalid, "头像仅支持 JPG、PNG、WebP 格式")
		return
	}

	u := currentUser(r)
	name := u.ID + ext
	relPath := filepath.ToSlash(filepath.Join(uploadsDirName, name))
	if err := os.WriteFile(filepath.Join(s.uploadsDir(), name), data, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, CodeAvatarInvalid, "保存头像失败")
		return
	}
	if err := s.store.UpdateAvatar(r.Context(), u.ID, relPath); err != nil {
		writeError(w, http.StatusInternalServerError, CodeAvatarInvalid, "保存头像失败")
		return
	}

	if old := u.Avatar; old != "" && old != relPath {
		_ = os.Remove(filepath.Join(s.cfg.DataDir, filepath.FromSlash(old)))
	}
	u.Avatar = relPath
	u.AvatarPreset = 0
	writeOK(w, toProfileResponse(u))
}

// handleAvatarPreset 设定用户选中的系统头像序号（preset ∈ [0,12]；0 表示清除 preset，
// 回到 hash 默认或上传头像）。同时清空上传头像路径以保证头像来源唯一。
func (s *Server) handleAvatarPreset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Preset int `json:"preset"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Preset < 0 || req.Preset > 12 {
		writeError(w, http.StatusBadRequest, CodeAvatarInvalid, "预设头像序号无效（0~12）")
		return
	}

	u := currentUser(r)
	if err := s.store.UpdateAvatarPreset(r.Context(), u.ID, req.Preset); err != nil {
		writeError(w, http.StatusInternalServerError, CodeAvatarInvalid, "保存头像失败")
		return
	}

	// 若之前有上传头像，清理磁盘文件
	if old := u.Avatar; old != "" {
		_ = os.Remove(filepath.Join(s.cfg.DataDir, filepath.FromSlash(old)))
	}
	u.Avatar = ""
	u.AvatarPreset = req.Preset
	writeOK(w, toProfileResponse(u))
}

type passwordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// handleChangePassword 修改密码：校验原密码后更新为新密码
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req passwordRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	u := currentUser(r)
	if !auth.VerifyPassword(req.OldPassword, u.PasswordHash, u.PasswordSalt) {
		writeError(w, http.StatusBadRequest, CodeOldPasswordWrong, "原密码错误")
		return
	}
	if err := auth.ValidatePassword(req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, CodePasswordInvalid, "密码需为 6-32 位且至少包含字母与数字")
		return
	}

	hash, salt, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodePasswordInvalid, "密码处理失败")
		return
	}
	if err := s.store.UpdatePassword(r.Context(), u.ID, hash, salt); err != nil {
		writeError(w, http.StatusInternalServerError, CodePasswordInvalid, "更新密码失败")
		return
	}
	writeOK(w, nil)
}
