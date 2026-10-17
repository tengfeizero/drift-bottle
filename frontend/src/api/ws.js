// WebSocket 客户端：自动注入凭证、应用层心跳、断线指数退避重连

import { getToken } from './http'

const BASE_DELAY = 1000
const MAX_DELAY = 30000
const PING_INTERVAL = 30000

/**
 * 创建实时连接管理器。
 * @param {(event: {type: string, data: any}) => void} onEvent 收到服务端推送时回调
 * @param {(connected: boolean) => void} onStatusChange 连接状态变化时回调
 */
export function createRealtime({ onEvent, onStatusChange } = {}) {
  let socket = null
  let stopped = true
  let retries = 0
  let reconnectTimer = null
  let pingTimer = null

  function notify(connected) {
    if (onStatusChange) onStatusChange(connected)
  }

  function stopPing() {
    if (pingTimer) {
      clearInterval(pingTimer)
      pingTimer = null
    }
  }

  function startPing() {
    stopPing()
    pingTimer = setInterval(() => send({ type: 'ping' }), PING_INTERVAL)
  }

  function scheduleReconnect() {
    retries += 1
    const delay = Math.min(BASE_DELAY * 2 ** (retries - 1), MAX_DELAY)
    notify(false)
    reconnectTimer = setTimeout(connect, delay)
  }

  function connect() {
    if (stopped) return
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    const token = getToken()
    if (!token) return

    const scheme = location.protocol === 'https:' ? 'wss' : 'ws'
    const url = `${scheme}://${location.host}/ws?token=${encodeURIComponent(token)}`

    try {
      socket = new WebSocket(url)
    } catch {
      scheduleReconnect()
      return
    }

    socket.onopen = () => {
      retries = 0
      notify(true)
      startPing()
    }

    socket.onmessage = (event) => {
      let payload
      try {
        payload = JSON.parse(event.data)
      } catch {
        return
      }
      if (payload?.type === 'pong') return
      if (onEvent) onEvent(payload)
    }

    socket.onclose = () => {
      stopPing()
      socket = null
      if (!stopped) scheduleReconnect()
    }

    // 连接错误后 onclose 会随即触发，统一在 onclose 中处理重连
    socket.onerror = () => {}
  }

  function send(message) {
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify(message))
    }
  }

  return {
    /** 开始连接（可重复调用，用于登录后重启） */
    start() {
      stopped = false
      retries = 0
      connect()
    },
    /** 主动断开并停止重连（退出登录 / 页面卸载时调用） */
    stop() {
      stopped = true
      stopPing()
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
      if (socket) {
        socket.onclose = null
        socket.close()
        socket = null
      }
      notify(false)
    },
  }
}
