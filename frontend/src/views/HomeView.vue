<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import AvatarBadge from '../components/AvatarBadge.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import ThrowBottleModal from '../components/ThrowBottleModal.vue'
import PickBottleModal from '../components/PickBottleModal.vue'
import BottleDetailModal from '../components/BottleDetailModal.vue'
import SettingsModal from '../components/SettingsModal.vue'
import PasswordModal from '../components/PasswordModal.vue'
import { useAuthStore } from '../stores/auth'
import { useBottleStore, BottleState } from '../stores/bottles'
import { useToastStore } from '../stores/toast'
import { createRealtime } from '../api/ws'
import { formatTime } from '../utils/format'
import { attachmentLabel } from '../utils/attachment'

const router = useRouter()
const auth = useAuthStore()
const bottles = useBottleStore()
const toast = useToastStore()

const showThrow = ref(false)
const showPick = ref(false)
const showSettings = ref(false)
const showPassword = ref(false)
const showLogout = ref(false)
const detailOpen = ref(false)
const detailId = ref(null)

let realtime = null

onMounted(async () => {
  try {
    await auth.loadMe()
  } catch {
    return // 登录态失效由全局处理器跳转登录页
  }
  await bottles.refresh().catch(() => {})

  realtime = createRealtime({
    onEvent: (event) => bottles.handleEvent(event),
    onStatusChange: (connected) => {
      bottles.connected = connected
    },
  })
  realtime.start()
})

onBeforeUnmount(() => realtime?.stop())

function openDetail(id) {
  detailId.value = id
  detailOpen.value = true
}

function closeDetail() {
  detailOpen.value = false
}

function openSettingsFromPick() {
  showPick.value = false
  showSettings.value = true
}

async function logout() {
  showLogout.value = false
  realtime?.stop()
  await auth.logout()
  toast.success('已退出登录')
  router.replace({ name: 'login' })
}

function titleOf(bottle) {
  if (bottle.peer_nickname) return bottle.peer_nickname
  if (bottle.state === BottleState.Sea) return '漂流中'
  return '未知对方'
}

function previewOf(bottle) {
  if (bottle.last_message) return bottle.last_message
  if (bottle.content) return bottle.content
  // 纯媒体内容：文字为空时用附件类型占位
  return bottle.attachment ? attachmentLabel(bottle.attachment.kind) : ''
}

function timeOf(bottle) {
  return formatTime(bottle.last_active_at || bottle.created_at)
}
</script>

<template>
  <div class="home">
    <header class="topbar">
      <div class="brand">
        <span class="brand__logo">🌊</span>
        <span>漂流瓶</span>
      </div>
      <div class="topbar__right">
        <!-- 用户头像：点击打开资料设置 -->
        <button class="user-chip" type="button" @click="showSettings = true">
          <AvatarBadge :src="auth.avatar" :uid="auth.id" :name="auth.nickname" :size="30" />
          <span class="user-chip__name">{{ auth.nickname }}</span>
          <i v-if="bottles.unreadCount" class="user-chip__badge">{{ bottles.unreadCount }}</i>
        </button>
        <!-- 密码修改 -->
        <button class="topbar-link" type="button" @click="showPassword = true">修改密码</button>
        <!-- 安全退出 -->
        <button class="topbar-link topbar-link--danger" type="button" @click="showLogout = true">
          安全退出
        </button>
      </div>
    </header>

    <main class="stage">
      <p class="stage__quota">
        今日剩余投放 <strong>{{ bottles.quotas.throw_remaining }}</strong> 次 · 捞取
        <strong>{{ bottles.quotas.pick_remaining }}</strong> 次
      </p>

      <div class="stage__actions">
        <button class="action action--throw" type="button" @click="showThrow = true">
          <span class="action__icon">🍾</span>
          <span class="action__label">扔瓶子</span>
        </button>
        <button class="action action--pick" type="button" @click="showPick = true">
          <span class="action__icon">🎣</span>
          <span class="action__label">捞瓶子</span>
        </button>
      </div>

      <p v-if="!auth.profileComplete" class="stage__tip" @click="showSettings = true">
        完善性别与年龄后才能捞瓶子，点此设置 ›
      </p>
      <p v-else-if="!bottles.connected" class="stage__tip stage__tip--muted">
        实时连接已断开，正在自动重连…
      </p>
    </main>

    <footer class="strip">
      <p v-if="!bottles.list.length && !bottles.loading" class="strip__empty">
        还没有瓶子，扔一个或捞一个试试吧
      </p>
      <ul v-else class="strip__list">
        <li v-for="bottle in bottles.list" :key="bottle.id">
          <button
            class="bottle-card"
            :class="{ 'is-unread': bottle.unread }"
            type="button"
            @click="openDetail(bottle.id)"
          >
            <AvatarBadge
              :src="bottle.peer_avatar"
              :uid="bottle.peer_id"
              :name="bottle.peer_nickname || '?'"
              :size="40"
            />
            <span class="bottle-card__name">{{ titleOf(bottle) }}</span>
            <span class="bottle-card__preview">{{ previewOf(bottle) }}</span>
            <span class="bottle-card__time">{{ timeOf(bottle) }}</span>
            <i v-if="bottle.unread" class="bottle-card__dot" />
          </button>
        </li>
      </ul>
    </footer>

    <ThrowBottleModal v-model:open="showThrow" />
    <PickBottleModal
      v-model:open="showPick"
      @picked="openDetail"
      @open-settings="openSettingsFromPick"
    />
    <BottleDetailModal :open="detailOpen" :bottle-id="detailId" @close="closeDetail" />
    <SettingsModal v-model:open="showSettings" />
    <PasswordModal v-model:open="showPassword" />

    <ConfirmDialog
      :open="showLogout"
      title="退出登录"
      message="退出后需要用 ID 和密码重新登录，确定要退出吗？"
      confirm-text="退出登录"
      danger
      @confirm="logout"
      @cancel="showLogout = false"
    />
  </div>
</template>

<style scoped>
.home {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 100svh;
}

/* ---------- 顶栏 ---------- */

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}

.brand {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 17px;
  font-weight: 600;
}

.brand__logo {
  font-size: 20px;
}

.topbar__right {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 顶栏功能入口：修改密码 / 安全退出 */
.topbar-link {
  padding: 5px 10px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface-2);
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
}

.topbar-link:active {
  transform: scale(0.96);
}

.topbar-link--danger {
  color: var(--danger);
}

.user-chip {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px 4px 4px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface-2);
}

.user-chip__name {
  max-width: 84px;
  overflow: hidden;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-chip__badge {
  position: absolute;
  top: -4px;
  right: -4px;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  border-radius: 999px;
  background: var(--danger);
  color: #fff;
  font-size: 11px;
  font-style: normal;
  line-height: 18px;
  text-align: center;
}

/* ---------- 中部操作区 ---------- */

.stage {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 22px;
  padding: 24px 16px;
}

.stage__quota {
  font-size: 13px;
  color: var(--muted);
}

.stage__quota strong {
  color: var(--primary-dark);
}

.stage__actions {
  display: flex;
  gap: 16px;
  width: 100%;
  max-width: 420px;
}

.action {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 26px 12px;
  border: none;
  border-radius: 18px;
  color: #fff;
  box-shadow: 0 10px 22px rgba(14, 165, 233, 0.25);
  transition: transform 0.12s;
}

.action:active {
  transform: scale(0.97);
}

.action--throw {
  background: linear-gradient(150deg, #0ea5e9, #0284c7);
}

.action--pick {
  background: linear-gradient(150deg, #38bdf8, #0ea5e9);
}

.action__icon {
  font-size: 30px;
}

.action__label {
  font-size: 16px;
  font-weight: 600;
}

.stage__tip {
  font-size: 13px;
  color: var(--primary-dark);
}

.stage__tip--muted {
  color: var(--muted);
}

/* ---------- 底部瓶子列表 ---------- */

.strip {
  padding: 12px 0 16px;
  background: var(--surface);
  border-top: 1px solid var(--border);
}

.strip__empty {
  padding: 20px 16px;
  font-size: 13px;
  text-align: center;
  color: var(--muted);
}

.strip__list {
  display: flex;
  gap: 10px;
  margin: 0;
  padding: 0 16px;
  list-style: none;
  overflow-x: auto;
}

.bottle-card {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  width: 152px;
  height: 132px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface-2);
  text-align: left;
}

.bottle-card.is-unread {
  border-color: var(--danger);
  background: var(--danger-soft);
}

.bottle-card__name {
  max-width: 100%;
  overflow: hidden;
  font-size: 13px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bottle-card__preview {
  display: -webkit-box;
  overflow: hidden;
  font-size: 12px;
  line-height: 1.4;
  color: var(--muted);
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  word-break: break-word;
}

.bottle-card__time {
  margin-top: auto;
  font-size: 11px;
  color: var(--muted);
}

.bottle-card__dot {
  position: absolute;
  top: 10px;
  right: 10px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--danger);
}

/* 窄屏顶栏：用户胶囊只保留头像，给功能入口腾位置 */
@media (max-width: 480px) {
  .topbar__right {
    gap: 6px;
  }

  .topbar-link {
    padding: 5px 8px;
  }

  .user-chip {
    padding-right: 4px;
  }

  .user-chip__name {
    display: none;
  }
}
</style>
