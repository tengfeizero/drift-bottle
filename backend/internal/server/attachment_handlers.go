package server

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"driftbottle/internal/store"
)

// attachmentsDirName 附件子目录名（位于上传目录之内）
const attachmentsDirName = "attachments"

// maxAttachmentSize 单个附件文件大小上限
const maxAttachmentSize = 50 << 20 // 50 MiB

// maxAttachmentBodySize 请求体上限（含 multipart 边界开销，略大于文件上限）
const maxAttachmentBodySize = maxAttachmentSize + (1 << 20)

// attachmentKindByExt 允许的附件扩展名与类型（音频/视频的魔数识别不可靠，按扩展名放行）
var attachmentKindByExt = map[string]string{
	".jpg":  store.AttachmentKindImage,
	".jpeg": store.AttachmentKindImage,
	".png":  store.AttachmentKindImage,
	".gif":  store.AttachmentKindImage,
	".webp": store.AttachmentKindImage,
	".bmp":  store.AttachmentKindImage,
	".mp3":  store.AttachmentKindAudio,
	".m4a":  store.AttachmentKindAudio,
	".aac":  store.AttachmentKindAudio,
	".wav":  store.AttachmentKindAudio,
	".ogg":  store.AttachmentKindAudio,
	".flac": store.AttachmentKindAudio,
	".mp4":  store.AttachmentKindVideo,
	".webm": store.AttachmentKindVideo,
	".mov":  store.AttachmentKindVideo,
}

// attachmentMIMEByExt 扩展名对应的 Content-Type（供静态目录正确响应，含视频拖动所需的 Range 支持）
var attachmentMIMEByExt = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".mp3":  "audio/mpeg",
	".m4a":  "audio/mp4",
	".aac":  "audio/aac",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".flac": "audio/flac",
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".mov":  "video/quicktime",
}

// imageContentTypes 图片魔数识别结果白名单
var imageContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	"image/bmp":  true,
}

// registerAttachmentMIMETypes 注册附件类型的 MIME 映射，避免静态服务因系统缺少映射而返回错误类型
func registerAttachmentMIMETypes() {
	for ext, ct := range attachmentMIMEByExt {
		_ = mime.AddExtensionType(ext, ct)
	}
}

// attachmentsDir 附件存储目录的绝对路径
func (s *Server) attachmentsDir() string {
	return filepath.Join(s.uploadsDir(), attachmentsDirName)
}

type attachmentResponse struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	Mime string `json:"mime"`
	Size int64  `json:"size"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func toAttachmentResponse(a *store.Attachment) *attachmentResponse {
	if a == nil {
		return nil
	}
	return &attachmentResponse{
		ID:   a.ID,
		Kind: a.Kind,
		Mime: a.Mime,
		Size: a.Size,
		Name: a.OrigName,
		URL:  "/" + a.Path,
	}
}

// attachmentLabel 附件在无文字内容时的摘要占位
func attachmentLabel(kind string) string {
	switch kind {
	case store.AttachmentKindImage:
		return "[图片]"
	case store.AttachmentKindAudio:
		return "[语音]"
	case store.AttachmentKindVideo:
		return "[视频]"
	}
	return "[附件]"
}

// previewText 生成列表摘要：内容为空但有附件时用附件类型标签占位
func previewText(content string, att *store.Attachment) string {
	if strings.TrimSpace(content) == "" && att != nil {
		return attachmentLabel(att.Kind)
	}
	return summarize(content)
}

// maxOrigNameLen 原文件名保存长度上限（rune）
const maxOrigNameLen = 80

// cleanOriginalName 仅保留文件名并截断，避免路径穿越与超长文本
func cleanOriginalName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if utf8.RuneCountInString(name) > maxOrigNameLen {
		name = string([]rune(name)[:maxOrigNameLen])
	}
	return name
}

// randomAttachmentName 生成随机文件名（不含扩展名）
func randomAttachmentName() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return strings.ReplaceAll(time.Now().Format("20060102150405.000000000"), ".", "")
	}
	return hex.EncodeToString(buf)
}

// removeUploadFiles 清理上传目录中的文件（传入相对数据目录的路径）
func (s *Server) removeUploadFiles(paths []string) {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if err := os.Remove(filepath.Join(s.cfg.DataDir, filepath.FromSlash(p))); err != nil && !os.IsNotExist(err) {
			log.Printf("清理附件文件 %s 失败: %v", p, err)
		}
	}
}

// handleUploadAttachment 上传附件（multipart 字段 file），返回可后续绑定的附件信息
func (s *Server) handleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAttachmentBodySize)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeAttachmentInvalid, "请上传不超过 50MB 的文件")
		return
	}
	defer func() { _ = file.Close() }()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	kind, ok := attachmentKindByExt[ext]
	if !ok {
		writeError(w, http.StatusBadRequest, CodeAttachmentInvalid, "仅支持图片、音频与视频文件")
		return
	}

	data, err := io.ReadAll(file)
	if err != nil || len(data) == 0 {
		writeError(w, http.StatusBadRequest, CodeAttachmentInvalid, "读取文件失败")
		return
	}
	if len(data) > maxAttachmentSize {
		writeError(w, http.StatusBadRequest, CodeAttachmentInvalid, "文件大小不能超过 50MB")
		return
	}

	contentType := attachmentMIMEByExt[ext]
	if kind == store.AttachmentKindImage {
		// 图片可被可靠识别，按实际内容校验并记录类型
		detected := http.DetectContentType(data)
		if !imageContentTypes[detected] {
			writeError(w, http.StatusBadRequest, CodeAttachmentInvalid, "图片文件格式不受支持")
			return
		}
		contentType = detected
	}

	name := randomAttachmentName() + ext
	if err := os.MkdirAll(s.attachmentsDir(), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, CodeAttachmentInvalid, "保存附件失败")
		return
	}
	if err := os.WriteFile(filepath.Join(s.attachmentsDir(), name), data, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, CodeAttachmentInvalid, "保存附件失败")
		return
	}

	a := &store.Attachment{
		OwnerID:   currentUser(r).ID,
		Kind:      kind,
		Mime:      contentType,
		Size:      int64(len(data)),
		OrigName:  cleanOriginalName(header.Filename),
		Path:      filepath.ToSlash(filepath.Join(uploadsDirName, attachmentsDirName, name)),
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := s.store.CreateAttachment(r.Context(), a); err != nil {
		_ = os.Remove(filepath.Join(s.attachmentsDir(), name))
		writeError(w, http.StatusInternalServerError, CodeAttachmentInvalid, "保存附件失败")
		return
	}
	writeOK(w, toAttachmentResponse(a))
}
