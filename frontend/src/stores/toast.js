import { defineStore } from 'pinia'

// 轻提示：统一承载操作反馈，3 秒后自动消失
export const useToastStore = defineStore('toast', {
  state: () => ({ items: [] }),
  actions: {
    push(message, type = 'info') {
      const id = `${Date.now()}-${Math.random()}`
      this.items.push({ id, message, type })
      setTimeout(() => this.dismiss(id), 3000)
    },
    success(message) {
      this.push(message, 'success')
    },
    error(message) {
      this.push(message, 'error')
    },
    dismiss(id) {
      this.items = this.items.filter((item) => item.id !== id)
    },
  },
})
