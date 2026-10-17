package server

import (
	"encoding/json"
	"net/http"
)

// 业务错误码：0 表示成功，非 0 表示业务错误
const (
	CodeOK                 = 0
	CodePasswordInvalid    = 1001 // PASSWORD_INVALID
	CodeIDOrPasswordWrong  = 1002 // ID_OR_PASSWORD_WRONG
	CodeUnauthorized       = 1003 // UNAUTHORIZED
	CodeIDGenerationFailed = 1004 // ID_GENERATION_FAILED
	CodeInvalidRequest     = 1005 // INVALID_REQUEST
	CodeProfileInvalid     = 1006 // PROFILE_INVALID
	CodeOldPasswordWrong   = 1007 // OLD_PASSWORD_WRONG
	CodeAvatarInvalid      = 1008 // AVATAR_INVALID
	CodeQuotaExceeded      = 1009 // QUOTA_EXCEEDED
	CodeBottleInvalid      = 1010 // BOTTLE_INVALID
	CodeNoMatch            = 1011 // NO_MATCH
	CodeProfileIncomplete  = 1012 // PROFILE_INCOMPLETE
	CodeBottleNotFound     = 1013 // BOTTLE_NOT_FOUND
	CodeAttachmentInvalid  = 1014 // ATTACHMENT_INVALID
)

// apiResponse 统一返回结构
type apiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func writeJSON(w http.ResponseWriter, status int, body apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeOK 返回成功响应
func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, apiResponse{Code: CodeOK, Message: "ok", Data: data})
}

// writeError 返回业务错误响应
func writeError(w http.ResponseWriter, status, code int, message string) {
	writeJSON(w, status, apiResponse{Code: code, Message: message})
}

// decodeJSON 解析请求体；失败时已写出错误响应并返回 false
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer func() { _ = r.Body.Close() }()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "请求格式错误")
		return false
	}
	return true
}
