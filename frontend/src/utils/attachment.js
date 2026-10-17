// 附件（图片 / 音频 / 视频）的共享常量与展示辅助

/** 单个附件大小上限：50MB */
export const MAX_ATTACHMENT_SIZE = 50 * 1024 * 1024

/** 文件选择框允许的扩展名（与后端白名单保持一致） */
export const ATTACHMENT_ACCEPT = [
  '.jpg',
  '.jpeg',
  '.png',
  '.gif',
  '.webp',
  '.bmp',
  '.mp3',
  '.m4a',
  '.aac',
  '.wav',
  '.ogg',
  '.flac',
  '.mp4',
  '.webm',
  '.mov',
].join(',')

const KIND_BY_EXT = {
  jpg: 'image',
  jpeg: 'image',
  png: 'image',
  gif: 'image',
  webp: 'image',
  bmp: 'image',
  mp3: 'audio',
  m4a: 'audio',
  aac: 'audio',
  wav: 'audio',
  ogg: 'audio',
  flac: 'audio',
  mp4: 'video',
  webm: 'video',
  mov: 'video',
}

/** 按扩展名推断附件类型；不支持的文件返回空串 */
export function attachmentKind(filename) {
  const dot = filename.lastIndexOf('.')
  if (dot < 0) return ''
  return KIND_BY_EXT[filename.slice(dot + 1).toLowerCase()] || ''
}

/** 附件类型 → 中文标签（无文字内容时的占位摘要） */
export function attachmentLabel(kind) {
  if (kind === 'image') return '[图片]'
  if (kind === 'audio') return '[语音]'
  if (kind === 'video') return '[视频]'
  return '[附件]'
}

/** 字节数 → 可读大小 */
export function formatFileSize(bytes) {
  const size = Number(bytes) || 0
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}
