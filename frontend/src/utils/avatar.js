// 系统默认头像（十二生肖）：未上传自定义头像的用户按其用户 ID 稳定分配一只动物
const modules = import.meta.glob('../assets/avatars/*.png', { eager: true, import: 'default' })

// 按生肖编号排序（文件名 01-rat … 12-pig）
export const defaultAvatars = Object.keys(modules)
  .sort()
  .map((key) => modules[key])

// djb2 字符串哈希，保证同一用户 ID 永远映射到同一只动物
function hashId(id) {
  let hash = 5381
  for (let i = 0; i < id.length; i += 1) {
    hash = ((hash << 5) + hash + id.charCodeAt(i)) >>> 0
  }
  return hash
}

// defaultAvatarOf 返回用户对应的默认头像 URL；无 ID 时返回空串
export function defaultAvatarOf(userId) {
  if (!userId) return ''
  return defaultAvatars[hashId(String(userId)) % defaultAvatars.length]
}
