import { createRouter, createWebHistory } from 'vue-router'
import LoginView from '../views/LoginView.vue'
import HomeView from '../views/HomeView.vue'
import { getToken } from '../api/http'

const routes = [
  {
    path: '/login',
    name: 'login',
    component: LoginView,
    meta: { guest: true },
  },
  {
    path: '/',
    name: 'home',
    component: HomeView,
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 路由守卫：未登录只能访问登录页，已登录不再回到登录页
router.beforeEach((to) => {
  const loggedIn = !!getToken()
  if (!loggedIn && !to.meta.guest) return { name: 'login' }
  if (loggedIn && to.meta.guest) return { name: 'home' }
  return true
})

export default router
