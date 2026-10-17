// 展示层格式化工具

const MINUTE = 60 * 1000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

const pad = (n) => String(n).padStart(2, '0')

/** 时间戳（毫秒）→ 友好时间文本 */
export function formatTime(ms) {
  if (!ms) return ''
  const date = new Date(ms)
  const diff = Date.now() - ms
  if (diff >= 0 && diff < MINUTE) return '刚刚'
  if (diff >= 0 && diff < HOUR) return `${Math.floor(diff / MINUTE)} 分钟前`

  const clock = `${pad(date.getHours())}:${pad(date.getMinutes())}`
  const today = new Date()
  if (today.toDateString() === date.toDateString()) return clock
  if (new Date(Date.now() - DAY).toDateString() === date.toDateString()) {
    return `昨天 ${clock}`
  }
  return `${date.getMonth() + 1}-${pad(date.getDate())} ${clock}`
}

/** 性别码 → 文案 */
export function genderLabel(gender) {
  if (gender === 1) return '男'
  if (gender === 2) return '女'
  return '未设置'
}

/** 对方资料摘要（性别 + 年龄） */
export function peerDescription(gender, age) {
  const parts = []
  if (gender === 1) parts.push('男')
  else if (gender === 2) parts.push('女')
  if (age > 0) parts.push(`${age} 岁`)
  return parts.join(' · ') || '资料未完善'
}
