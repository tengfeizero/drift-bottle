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

// 每日投放/捞取上限（共享规则 R-01）
const (
	throwDailyLimit = 99
	pickDailyLimit  = 999
)

// 投放瓶子字段默认值与取值范围
const (
	bottleContentMaxLen = 500
	bottleAgeMin        = 1
	bottleAgeMax        = 150
	defaultTargetAgeMin = 18
	defaultTargetAgeMax = 99
)

// shanghai 时区（Asia/Shanghai 无夏令时，固定 +8）
var shanghai = time.FixedZone("CST", 8*3600)

// today 返回 Asia/Shanghai 自然日日期字符串
func today() string {
	return time.Now().In(shanghai).Format(store.DateFormat)
}

type createBottleRequest struct {
	Content      string `json:"content"`
	AttachmentID int64  `json:"attachment_id"`
	TargetGender int    `json:"target_gender"`
	TargetAgeMin int    `json:"target_age_min"`
	TargetAgeMax int    `json:"target_age_max"`
}

type bottleResponse struct {
	ID           int64               `json:"id"`
	Content      string              `json:"content"`
	Attachment   *attachmentResponse `json:"attachment"`
	TargetGender int                 `json:"target_gender"`
	TargetAgeMin int                 `json:"target_age_min"`
	TargetAgeMax int                 `json:"target_age_max"`
	State        int                 `json:"state"`
	CreatedAt    int64               `json:"created_at"`
}

func toBottleResponse(b *store.Bottle, attachment *store.Attachment) bottleResponse {
	return bottleResponse{
		ID:           b.ID,
		Content:      b.Content,
		Attachment:   toAttachmentResponse(attachment),
		TargetGender: b.TargetGender,
		TargetAgeMin: b.TargetAgeMin,
		TargetAgeMax: b.TargetAgeMax,
		State:        b.State,
		CreatedAt:    b.CreatedAt,
	}
}

// handleCreateBottle 投放瓶子：校验字段并扣减当日投放配额
func (s *Server) handleCreateBottle(w http.ResponseWriter, r *http.Request) {
	var req createBottleRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	content := strings.TrimSpace(req.Content)
	// 带附件时文字可为空（纯媒体投放）；无附件时仍需 1-500 字
	if n := utf8.RuneCountInString(content); n > bottleContentMaxLen || (n < 1 && req.AttachmentID == 0) {
		writeError(w, http.StatusBadRequest, CodeBottleInvalid, "投放内容需为 1-500 字，或携带一个附件")
		return
	}
	if req.TargetGender != store.GenderUnset && req.TargetGender != store.GenderMale && req.TargetGender != store.GenderFemale {
		writeError(w, http.StatusBadRequest, CodeBottleInvalid, "目标性别取值无效")
		return
	}

	// 未提供（0）时采用默认年龄范围
	ageMin, ageMax := req.TargetAgeMin, req.TargetAgeMax
	if ageMin == 0 {
		ageMin = defaultTargetAgeMin
	}
	if ageMax == 0 {
		ageMax = defaultTargetAgeMax
	}
	if ageMin < bottleAgeMin || ageMin > bottleAgeMax || ageMax < bottleAgeMin || ageMax > bottleAgeMax {
		writeError(w, http.StatusBadRequest, CodeBottleInvalid, "目标年龄范围需为 1-150")
		return
	}
	if ageMin > ageMax {
		writeError(w, http.StatusBadRequest, CodeBottleInvalid, "目标年龄下限不能大于上限")
		return
	}

	u := currentUser(r)
	b := &store.Bottle{
		OwnerID:      u.ID,
		Content:      content,
		TargetGender: req.TargetGender,
		TargetAgeMin: ageMin,
		TargetAgeMax: ageMax,
		State:        store.BottleStateSea,
		CreatedAt:    time.Now().UnixMilli(),
	}
	err := s.store.CreateBottleWithQuota(r.Context(), b, req.AttachmentID, today(), throwDailyLimit)
	if errors.Is(err, store.ErrQuotaExceeded) {
		writeError(w, http.StatusBadRequest, CodeQuotaExceeded, "今日投放次数已用完")
		return
	}
	if errors.Is(err, store.ErrAttachmentInvalid) {
		writeError(w, http.StatusBadRequest, CodeAttachmentInvalid, "附件无效或已被使用")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeBottleInvalid, "投放失败")
		return
	}
	attachment, _ := s.store.AttachmentByBottleID(r.Context(), b.ID)
	writeOK(w, toBottleResponse(b, attachment))
}

// pickedBottleResponse 捞取结果：瓶子内容 + 投放者资料（供详情窗口展示）
type pickedBottleResponse struct {
	bottleResponse
	OwnerNickname string `json:"owner_nickname"`
	OwnerAvatar   string `json:"owner_avatar"`
	OwnerGender   int    `json:"owner_gender"`
	OwnerAge      int    `json:"owner_age"`
}

// handlePickBottle 捞取瓶子：资料完整性校验 → 扣减配额 → 匹配并置 HELD
func (s *Server) handlePickBottle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u.Gender == store.GenderUnset || u.Age == 0 {
		writeError(w, http.StatusBadRequest, CodeProfileIncomplete, "请先设置性别和年龄")
		return
	}

	b, err := s.store.PickBottleWithQuota(r.Context(), u.Age, u.Gender, u.ID, today(), pickDailyLimit)
	if errors.Is(err, store.ErrQuotaExceeded) {
		writeError(w, http.StatusBadRequest, CodeQuotaExceeded, "今日捞取次数已用完")
		return
	}
	if errors.Is(err, store.ErrNoMatch) {
		writeError(w, http.StatusNotFound, CodeNoMatch, "海里暂时没有符合条件的瓶子")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeBottleNotFound, "捞取失败")
		return
	}

	attachment, _ := s.store.AttachmentByBottleID(r.Context(), b.ID)
	resp := pickedBottleResponse{bottleResponse: toBottleResponse(b, attachment)}
	if owner, err := s.store.GetUserByID(r.Context(), b.OwnerID); err == nil {
		resp.OwnerNickname = owner.Nickname
		if owner.Avatar != "" {
			resp.OwnerAvatar = "/" + owner.Avatar
		}
		resp.OwnerGender = owner.Gender
		resp.OwnerAge = owner.Age
	}
	// 通知投放者瓶子被捞取（对方视角为当前捞取者）
	s.pushBottleUpdate(b.OwnerID, b.ID, b.State, peerFrom(u))
	writeOK(w, resp)
}

// handleReturnBottle 扔回海里：HELD → SEA，清空捞取者（不退还已消耗配额）
func (s *Server) handleReturnBottle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "瓶子 ID 非法")
		return
	}
	u := currentUser(r)
	ownerID := ""
	if b, err := s.store.GetBottleByID(r.Context(), id); err == nil {
		ownerID = b.OwnerID
	}
	if err := s.store.ReturnBottle(r.Context(), id, u.ID); err != nil {
		if errors.Is(err, store.ErrBottleNotFound) {
			writeError(w, http.StatusNotFound, CodeBottleNotFound, "瓶子不存在或无法扔回")
			return
		}
		writeError(w, http.StatusInternalServerError, CodeBottleNotFound, "扔回失败")
		return
	}
	// 通知投放者瓶子回到海里（已无捞取者，对方信息为空）
	s.pushBottleUpdate(ownerID, id, store.BottleStateSea, nil)
	writeOK(w, nil)
}

// handleQuotas 今日剩余投放/捞取次数
func (s *Server) handleQuotas(w http.ResponseWriter, r *http.Request) {
	q, err := s.store.GetQuota(r.Context(), currentUser(r).ID, today())
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInvalidRequest, "读取配额失败")
		return
	}
	writeOK(w, map[string]int{
		"throw_remaining": throwDailyLimit - q.Throw,
		"pick_remaining":  pickDailyLimit - q.Pick,
	})
}
