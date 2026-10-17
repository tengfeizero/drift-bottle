import { defineStore } from 'pinia'
import { http, getToken, setToken } from '../api/http'

const AVATAR = (path) => path || ''

// 账户与资料状态：凭证持久化在 localStorage，资料由 /api/me 拉取
export const useAuthStore = defineStore('auth', {
  state: () => ({
    id: '',
    nickname: '',
    avatar: '',
    avatarPreset: 0,
    gender: 0,
    age: 0,
    token: getToken(),
    loaded: false,
  }),
  getters: {
    isLoggedIn: (state) => !!state.token,
    // 性别与年龄均已设置才可捞瓶（共享规则 R-04）
    profileComplete: (state) => state.gender !== 0 && state.age > 0,
  },
  actions: {
    applyProfile(profile) {
      this.id = profile.id
      this.nickname = profile.nickname
      this.avatar = AVATAR(profile.avatar)
      this.avatarPreset = profile.avatar_preset || 0
      this.gender = profile.gender
      this.age = profile.age
      this.loaded = true
    },
    async register(password) {
      const data = await http.post('/api/register', { password })
      this.id = data.id
      this.nickname = data.nickname
      this.token = data.token
      setToken(data.token)
      this.loaded = false
      return data
    },
    async login(id, password) {
      const data = await http.post('/api/login', { id, password })
      this.id = data.id
      this.nickname = data.nickname
      this.token = data.token
      setToken(data.token)
      this.loaded = false
      return data
    },
    async loadMe() {
      this.applyProfile(await http.get('/api/me'))
      return this.$state
    },
    async updateProfile({ nickname, gender, age }) {
      const profile = await http.put('/api/me/profile', { nickname, gender, age })
      this.applyProfile(profile)
    },
    async uploadAvatar(file) {
      const form = new FormData()
      form.append('avatar', file)
      const profile = await http.postForm('/api/me/avatar', form)
      this.applyProfile(profile)
    },
    async setAvatarPreset(preset) {
      const profile = await http.put('/api/me/avatar-preset', { preset })
      this.applyProfile(profile)
    },
    async changePassword(oldPassword, newPassword) {
      await http.put('/api/me/password', {
        old_password: oldPassword,
        new_password: newPassword,
      })
    },
    async logout() {
      try {
        await http.post('/api/logout')
      } catch {
        // 忽略：服务端失败也照常清理本地登录态
      }
      this.reset()
    },
    reset() {
      this.id = ''
      this.nickname = ''
      this.avatar = ''
      this.gender = 0
      this.age = 0
      this.token = ''
      this.loaded = false
      setToken('')
    },
  },
})
