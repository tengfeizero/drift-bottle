import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './style.css'
import App from './App.vue'
import router from './router'
import { setUnauthorizedHandler } from './api/http'
import { useAuthStore } from './stores/auth'
import { useToastStore } from './stores/toast'

const app = createApp(App)
app.use(createPinia())
app.use(router)

// 登录态失效（凭证过期 / 被退出）：清理本地状态并回到登录页
setUnauthorizedHandler(() => {
  useAuthStore().reset()
  useToastStore().error('登录已过期，请重新登录')
  if (router.currentRoute.value.name !== 'login') {
    router.replace({ name: 'login' })
  }
})

app.mount('#app')
