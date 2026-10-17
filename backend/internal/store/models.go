package store

import "database/sql"

// 瓶子状态
const (
	BottleStateSea      = 0 // 海上（等待捞取）
	BottleStateHeld     = 1 // 持有中（已捞取未回复）
	BottleStateChatting = 2 // 会话中
)

// 性别取值
const (
	GenderUnset  = 0 // 未设置
	GenderMale   = 1 // 男
	GenderFemale = 2 // 女
)

// Bottle 瓶子
type Bottle struct {
	ID            int64
	OwnerID       string
	HolderID      sql.NullString
	Content       string
	TargetGender  int
	TargetAgeMin  int
	TargetAgeMax  int
	State         int
	CreatedAt     int64
	LastMessageAt sql.NullInt64
}
