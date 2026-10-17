// HTTP 客户端封装：统一注入 Bearer 凭证、解析统一响应、集中处理登录态失效

const TOKEN_KEY = 'driftbottle.token'

// 与后端 response.go 中的错误码保持一致
export const ErrorCode = {
  ID_OR_PASSWORD_WRONG: 1002,
  UNAUTHORIZED: 1003,
  QUOTA_EXCEEDED: 1009,
  NO_MATCH: 1011,
  PROFILE_INCOMPLETE: 1012,
  BOTTLE_NOT_FOUND: 1013,
  ATTACHMENT_INVALID: 1014,
}

let unauthorizedHandler = null

/** 注册登录态失效（401 UNAUTHORIZED）处理器；由 main.js 启动时注入 */
export function setUnauthorizedHandler(handler) {
  unauthorizedHandler = handler
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

/** 统一的业务/网络错误 */
export class ApiError extends Error {
  constructor(code, message, status = 0) {
    super(message || '请求失败')
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

async function request(method, url, { body, isForm = false } = {}) {
  const headers = {}
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`

  let payload
  if (body !== undefined) {
    if (isForm) {
      payload = body
    } else {
      headers['Content-Type'] = 'application/json'
      payload = JSON.stringify(body)
    }
  }

  let res
  try {
    res = await fetch(url, { method, headers, body: payload })
  } catch {
    throw new ApiError(-1, '网络异常，请检查网络后重试')
  }

  let data = null
  try {
    data = await res.json()
  } catch {
    data = null
  }

  // 仅"登录态失效"清空凭证并跳登录页；登录失败（1002）不触发
  if (res.status === 401 && data?.code === ErrorCode.UNAUTHORIZED) {
    setToken('')
    if (unauthorizedHandler) unauthorizedHandler()
    throw new ApiError(ErrorCode.UNAUTHORIZED, data?.message || '未登录或登录已过期', 401)
  }

  if (!data || data.code !== 0) {
    throw new ApiError(data?.code ?? res.status, data?.message || `请求失败（${res.status}）`, res.status)
  }
  return data.data
}

export const http = {
  get: (url) => request('GET', url),
  post: (url, body) => request('POST', url, { body }),
  put: (url, body) => request('PUT', url, { body }),
  del: (url) => request('DELETE', url),
  postForm: (url, formData) => request('POST', url, { body: formData, isForm: true }),
}
