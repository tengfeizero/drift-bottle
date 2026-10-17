package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"driftbottle/internal/store"
)

// 消息长度上限
const messageMaxLen = 500

// summarizeMaxLen 列表摘要截断长度
const summarizeMaxLen = 50

// peerIDOf 返回瓶子对话中"对方"的用户 ID；尚无对方（如无人捞取的 SEA 瓶子）时返回空串。
func peerIDOf(b *store.Bottle, meID string) string {
	if b.OwnerID == meID {
		if b.HolderID.Valid {
			return b.HolderID.String
		}
		return ""
	}
	if b.HolderID.Valid && b.HolderID.String == meID {
		return b.OwnerID
	}
	return ""
}

func isParticipant(b *store.Bottle, userID string) bool {
	return b.OwnerID == userID || (b.HolderID.Valid && b.HolderID.String == userID)
}

func summarize(content string) string {
	r := []rune(content)
	if len(r) <= summarizeMaxLen {
		return content
	}
	return string(r[:summarizeMaxLen]) + "…"
}

type listItemResponse struct {
	ID           int64               `json:"id"`
	State        int                 `json:"state"`
	Content      string              `json:"content"`
	Attachment   *attachmentResponse `json:"attachment"`
	CreatedAt    int64               `json:"created_at"`
	LastActiveAt int64               `json:"last_active_at"`
	PeerID       string              `json:"peer_id"`
	PeerNickname string              `json:"peer_nickname"`
	PeerAvatar   string              `json:"peer_avatar"`
	PeerGender   int                 `json:"peer_gender"`
	PeerAge      int                 `json:"peer_age"`
	LastMessage  string              `json:"last_message"`
	Unread       bool                `json:"unread"`
}

type messageResponse struct {
	ID         int64               `json:"id"`
	SenderID   string              `json:"sender_id"`
	Content    string              `json:"content"`
	Attachment *attachmentResponse `json:"attachment"`
	CreatedAt  int64               `json:"created_at"`
	ExpireAt   int64               `json:"expire_at"`
	Mine       bool                `json:"mine"`
}

type bottleDetailResponse struct {
	ID           int64               `json:"id"`
	State        int                 `json:"state"`
	Content      string              `json:"content"`
	Attachment   *attachmentResponse `json:"attachment"`
	CreatedAt    int64               `json:"created_at"`
	PeerID       string              `json:"peer_id"`
	PeerNickname string              `json:"peer_nickname"`
	PeerAvatar   string              `json:"peer_avatar"`
	PeerGender   int                 `json:"peer_gender"`
	PeerAge      int                 `json:"peer_age"`
	Messages     []messageResponse   `json:"messages"`
}

// handleListBottles 瓶子列表：我方参与的全部瓶子，含对方信息、最近消息摘要与未读标记。
func (s *Server) handleListBottles(w http.ResponseWriter, r *http.Request) {
	me := currentUser(r)
	bottles, err := s.store.ListBottles(r.Context(), me.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeBottleNotFound, "读取列表失败")
		return
	}

	now := time.Now().UnixMilli()
	bottleIDs := make([]int64, 0, len(bottles))
	for _, b := range bottles {
		bottleIDs = append(bottleIDs, b.ID)
	}
	attachments, _ := s.store.AttachmentsByBottleIDs(r.Context(), bottleIDs)

	items := make([]listItemResponse, 0, len(bottles))
	for _, b := range bottles {
		item := listItemResponse{
			ID:           b.ID,
			State:        b.State,
			Content:      b.Content,
			Attachment:   toAttachmentResponse(attachments[b.ID]),
			CreatedAt:    b.CreatedAt,
			LastActiveAt: b.CreatedAt,
		}
		if b.LastMessageAt.Valid {
			item.LastActiveAt = b.LastMessageAt.Int64
		}
		if peerID := peerIDOf(b, me.ID); peerID != "" {
			item.PeerID = peerID
			if peer, err := s.store.GetUserByID(r.Context(), peerID); err == nil {
				item.PeerNickname = peer.Nickname
				if peer.Avatar != "" {
					item.PeerAvatar = "/" + peer.Avatar
				}
				item.PeerGender, item.PeerAge = peer.Gender, peer.Age
			}
		}
		if msg, err := s.store.LastVisibleMessage(r.Context(), b.ID, now); err == nil && msg != nil {
			msgAttachment, _ := s.store.AttachmentByMessageID(r.Context(), msg.ID)
			item.LastMessage = previewText(msg.Content, msgAttachment)
		}
		if unread, err := s.store.HasUnread(r.Context(), b.ID, me.ID, now); err == nil {
			item.Unread = unread
		}
		items = append(items, item)
	}
	writeOK(w, items)
}

// handleBottleDetail 瓶子详情：对方信息 + 未过期消息，并清除未读标记。
func (s *Server) handleBottleDetail(w http.ResponseWriter, r *http.Request) {
	me := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "瓶子 ID 非法")
		return
	}

	b, err := s.store.GetBottleByID(r.Context(), id)
	if err != nil || !isParticipant(b, me.ID) {
		writeError(w, http.StatusNotFound, CodeBottleNotFound, "瓶子不存在")
		return
	}
	// HELD 期间仅捞取者可见，投放者此时不可见
	if b.OwnerID == me.ID && b.State == store.BottleStateHeld {
		writeError(w, http.StatusNotFound, CodeBottleNotFound, "瓶子不存在")
		return
	}

	now := time.Now().UnixMilli()
	msgs, err := s.store.ListMessages(r.Context(), id, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeBottleNotFound, "读取消息失败")
		return
	}
	if err := s.store.MarkRead(r.Context(), id, me.ID, now); err != nil {
		writeError(w, http.StatusInternalServerError, CodeBottleNotFound, "更新已读失败")
		return
	}

	bottleAttachment, _ := s.store.AttachmentByBottleID(r.Context(), id)
	detail := bottleDetailResponse{
		ID: b.ID, State: b.State, Content: b.Content,
		Attachment: toAttachmentResponse(bottleAttachment), CreatedAt: b.CreatedAt,
	}
	if peerID := peerIDOf(b, me.ID); peerID != "" {
		detail.PeerID = peerID
		if peer, err := s.store.GetUserByID(r.Context(), peerID); err == nil {
			detail.PeerNickname = peer.Nickname
			if peer.Avatar != "" {
				detail.PeerAvatar = "/" + peer.Avatar
			}
			detail.PeerGender, detail.PeerAge = peer.Gender, peer.Age
		}
	}
	messageIDs := make([]int64, 0, len(msgs))
	for _, m := range msgs {
		messageIDs = append(messageIDs, m.ID)
	}
	msgAttachments, _ := s.store.AttachmentsByMessageIDs(r.Context(), messageIDs)
	detail.Messages = make([]messageResponse, 0, len(msgs))
	for _, m := range msgs {
		detail.Messages = append(detail.Messages, messageResponse{
			ID: m.ID, SenderID: m.SenderID, Content: m.Content,
			Attachment: toAttachmentResponse(msgAttachments[m.ID]),
			CreatedAt:  m.CreatedAt, ExpireAt: m.ExpireAt, Mine: m.SenderID == me.ID,
		})
	}
	writeOK(w, detail)
}

// handleDeleteBottle 删除瓶子（双删）：SEA 状态下仅投放者可删除；HELD/CHATTING 任一方删除即彻底删除。
func (s *Server) handleDeleteBottle(w http.ResponseWriter, r *http.Request) {
	me := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "瓶子 ID 非法")
		return
	}

	// 先取瓶子以确定另一方；非参与者取不到对方，删除会被拒绝
	peerID := ""
	if b, err := s.store.GetBottleByID(r.Context(), id); err == nil {
		peerID = peerIDOf(b, me.ID)
	}

	paths, err := s.store.DeleteBottle(r.Context(), id, me.ID)
	if errors.Is(err, store.ErrBottleNotFound) {
		writeError(w, http.StatusNotFound, CodeBottleNotFound, "瓶子不存在或无权删除")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeBottleNotFound, "删除失败")
		return
	}
	s.removeUploadFiles(paths)

	// 双删后通知另一方移除该瓶子
	if peerID != "" {
		s.ws.push(peerID, wsEventBottleDeleted, wsBottleDeletedPayload{BottleID: id})
	}
	writeOK(w, nil)
}

type messageRequest struct {
	Content      string `json:"content"`
	AttachmentID int64  `json:"attachment_id"`
}

// handleSendMessage 发送消息：仅参与双方可发送，HELD 首次回复后转为 CHATTING。
func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	me := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "瓶子 ID 非法")
		return
	}

	var req messageRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	content := strings.TrimSpace(req.Content)
	// 带附件时文字可为空（纯媒体消息）；无附件时仍需 1-500 字
	if n := utf8.RuneCountInString(content); n > messageMaxLen || (n < 1 && req.AttachmentID == 0) {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "消息需为 1-500 字，或携带一个附件")
		return
	}

	b, err := s.store.GetBottleByID(r.Context(), id)
	if err != nil || !isParticipant(b, me.ID) || b.State == store.BottleStateSea {
		writeError(w, http.StatusForbidden, CodeBottleNotFound, "无权操作该瓶子")
		return
	}

	m, err := s.store.SendMessage(r.Context(), id, me.ID, content, req.AttachmentID, time.Now())
	if errors.Is(err, store.ErrBottleNotFound) {
		writeError(w, http.StatusNotFound, CodeBottleNotFound, "瓶子不存在")
		return
	}
	if errors.Is(err, store.ErrNotParticipant) {
		writeError(w, http.StatusForbidden, CodeBottleNotFound, "无权操作该瓶子")
		return
	}
	if errors.Is(err, store.ErrAttachmentInvalid) {
		writeError(w, http.StatusBadRequest, CodeAttachmentInvalid, "附件无效或已被使用")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeBottleNotFound, "发送失败")
		return
	}

	attachment, _ := s.store.AttachmentByMessageID(r.Context(), m.ID)
	att := toAttachmentResponse(attachment)

	// 实时推送新消息给另一方；HELD 首次回复时同时推送状态变更
	if peerID := peerIDOf(b, me.ID); peerID != "" {
		s.ws.push(peerID, wsEventMessage, wsMessagePayload{
			BottleID: id,
			Message: messageResponse{
				ID: m.ID, SenderID: me.ID, Content: m.Content, Attachment: att,
				CreatedAt: m.CreatedAt, ExpireAt: m.ExpireAt, Mine: false,
			},
		})
		if b.State == store.BottleStateHeld {
			s.pushBottleUpdate(peerID, id, store.BottleStateChatting, peerFrom(me))
		}
	}

	writeOK(w, messageResponse{
		ID: m.ID, SenderID: m.SenderID, Content: m.Content, Attachment: att,
		CreatedAt: m.CreatedAt, ExpireAt: m.ExpireAt, Mine: true,
	})
}
