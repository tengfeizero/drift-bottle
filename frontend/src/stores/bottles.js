import { defineStore } from 'pinia'
import { http } from '../api/http'
import { attachmentLabel } from '../utils/attachment'

// 瓶子状态机（对应后端 state）
export const BottleState = {
  Sea: 0,
  Held: 1,
  Chatting: 2,
}

// 瓶子列表、配额与实时事件联动
export const useBottleStore = defineStore('bottles', {
  state: () => ({
    list: [],
    quotas: { throw_remaining: 99, pick_remaining: 999 },
    loading: false,
    connected: false,
    // 最近一次实时事件信号：供详情窗口按需刷新（seq 保证同一事件只触发一次）
    eventSignal: null,
    eventSeq: 0,
  }),
  getters: {
    unreadCount: (state) => state.list.filter((item) => item.unread).length,
  },
  actions: {
    async loadList() {
      this.loading = true
      try {
        this.list = await http.get('/api/bottles')
      } finally {
        this.loading = false
      }
    },
    async loadQuotas() {
      this.quotas = await http.get('/api/quotas')
    },
    async refresh() {
      await Promise.all([this.loadList(), this.loadQuotas()])
    },
    async throwBottle(payload) {
      const bottle = await http.post('/api/bottles', payload)
      await this.refresh()
      return bottle
    },
    /** 上传附件，返回 { id, kind, mime, size, name, url }，供投放/发消息时绑定 */
    async uploadAttachment(file) {
      const form = new FormData()
      form.append('file', file)
      return http.postForm('/api/attachments', form)
    },
    async pickBottle() {
      const result = await http.post('/api/bottles/pick')
      await this.refresh()
      return result
    },
    async returnBottle(id) {
      await http.post(`/api/bottles/${id}/return`)
      this.remove(id)
      await this.loadQuotas()
    },
    async fetchDetail(id) {
      const detail = await http.get(`/api/bottles/${id}`)
      this.markRead(id) // 打开即清除未读（服务端已在详情接口内标记）
      return detail
    },
    async sendMessage(id, content, attachmentId = 0) {
      const message = await http.post(`/api/bottles/${id}/messages`, {
        content,
        attachment_id: attachmentId,
      })
      const item = this.list.find((bottle) => bottle.id === id)
      if (item) {
        item.last_message = content || attachmentLabel(message.attachment?.kind)
        item.last_active_at = message.created_at
        if (item.state === BottleState.Held) item.state = BottleState.Chatting
      }
      return message
    },
    async deleteBottle(id) {
      await http.del(`/api/bottles/${id}`)
      this.remove(id)
    },
    markRead(id) {
      const item = this.list.find((bottle) => bottle.id === id)
      if (item) item.unread = false
    },
    remove(id) {
      this.list = this.list.filter((bottle) => bottle.id !== id)
    },
    /** 处理服务端实时推送：刷新列表并把事件转发给详情窗口 */
    handleEvent(event) {
      const data = event?.data || {}
      switch (event?.type) {
        case 'message':
          this.eventSignal = {
            type: 'message',
            bottleId: data.bottleId,
            seq: (this.eventSeq += 1),
          }
          this.loadList()
          break
        case 'bottle_update':
          this.eventSignal = {
            type: 'bottle_update',
            bottleId: data.bottleId,
            state: data.state,
            seq: (this.eventSeq += 1),
          }
          this.loadList()
          break
        case 'bottle_deleted':
          this.remove(data.bottleId)
          this.eventSignal = {
            type: 'bottle_deleted',
            bottleId: data.bottleId,
            seq: (this.eventSeq += 1),
          }
          break
        default:
          break
      }
    },
  },
})
