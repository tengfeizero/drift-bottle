<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AvatarBadge from '../components/AvatarBadge.vue'
import AppModal from '../components/AppModal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import ThrowBottleModal from '../components/ThrowBottleModal.vue'
import PickBottleModal from '../components/PickBottleModal.vue'
import BottleDetailModal from '../components/BottleDetailModal.vue'
import SettingsModal from '../components/SettingsModal.vue'
import PasswordModal from '../components/PasswordModal.vue'
import AuthModal from '../components/AuthModal.vue'
import { useAuthStore } from '../stores/auth'
import { useBottleStore, BottleState } from '../stores/bottles'
import { useToastStore } from '../stores/toast'
import { createRealtime } from '../api/ws'
import { formatTime } from '../utils/format'
import { attachmentLabel } from '../utils/attachment'

const auth = useAuthStore()
const bottles = useBottleStore()
const toast = useToastStore()

const showThrow = ref(false)
const showPick = ref(false)
const showSettings = ref(false)
const showPassword = ref(false)
const showLogout = ref(false)
const showAuth = ref(false)
// 登录成功后强制展示防骗提醒；注册流程需等用户确认 ID 并关闭弹窗后再展示
const showWarning = ref(false)
const warningPending = ref(false)
const detailOpen = ref(false)
const detailId = ref(null)

let realtime = null

onMounted(async () => {
  if (auth.isLoggedIn) await initSession()
})

onBeforeUnmount(stopRealtime)

async function initSession() {
  try {
    await auth.loadMe()
  } catch {
    return // 登录态失效由全局处理器清理
  }
  await bottles.refresh().catch(() => {})
  startRealtime()
}

function startRealtime() {
  if (realtime) return
  realtime = createRealtime({
    onEvent: (event) => bottles.handleEvent(event),
    onStatusChange: (connected) => {
      bottles.connected = connected
    },
  })
  realtime.start()
}

function stopRealtime() {
  realtime?.stop()
  realtime = null
}

function openAuth() {
  showAuth.value = true
}

async function onAuthSuccess() {
  await initSession()
  // 登录弹窗通常已关闭；注册弹窗仍开着时，等用户确认后由 watch 触发
  if (showAuth.value) warningPending.value = true
  else showWarning.value = true
}

watch(showAuth, (open) => {
  if (!open && warningPending.value) {
    warningPending.value = false
    showWarning.value = true
  }
})

// 登录态失效或退出后：停掉实时连接并清空瓶子数据
watch(
  () => auth.isLoggedIn,
  (loggedIn) => {
    if (loggedIn) return
    stopRealtime()
    bottles.$reset()
  },
)

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
  stopRealtime()
  await auth.logout()
  toast.success('已退出登录')
}

// 未登录时点投放/捞取，先引导登录
function openThrow() {
  if (auth.isLoggedIn) showThrow.value = true
  else openAuth()
}

function openPick() {
  if (auth.isLoggedIn) showPick.value = true
  else openAuth()
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
        <template v-if="auth.isLoggedIn">
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
        </template>
        <!-- 未登录：点击打开登录 / 注册弹窗 -->
        <button v-else class="topbar-link topbar-link--primary" type="button" @click="openAuth">
          登录
        </button>
      </div>
    </header>

    <main class="stage">
      <p v-if="auth.isLoggedIn" class="stage__quota">
        今日剩余投放 <strong>{{ bottles.quotas.throw_remaining }}</strong> 次 · 捞取
        <strong>{{ bottles.quotas.pick_remaining }}</strong> 次
      </p>

      <div class="stage__actions">
        <button class="action action--throw" type="button" @click="openThrow">
          <span class="action__icon">🍾</span>
          <span class="action__label">扔瓶子</span>
        </button>
        <button class="action action--pick" type="button" @click="openPick">
          <span class="action__icon">🎣</span>
          <span class="action__label">捞瓶子</span>
        </button>
      </div>

      <p v-if="auth.isLoggedIn && !auth.profileComplete" class="stage__tip" @click="showSettings = true">
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
    <AuthModal v-model:open="showAuth" @success="onAuthSuccess" />

    <!-- 登录后的防骗提醒：不可跳过，需用户确认 -->
    <AppModal :open="showWarning" title="防骗提醒" :closable="false">
      <p class="safety__lead">网络交友有风险，请务必保护好自己的人身与财产安全。</p>
      <ul class="safety__list">
        <li>不要轻信网友的身份、照片和说辞，谨防冒充与情感诈骗。</li>
        <li>不向对方转账、发红包、代付或充值，拒绝投资、博彩、刷单等邀请。</li>
        <li>不点击陌生链接，不扫描来历不明的二维码，不安装对方推荐的 App。</li>
        <li>不透露身份证号、银行卡号、验证码、家庭住址等个人信息。</li>
        <li>一旦对方索要钱财或诱导投资，请立即停止联系并拉黑。</li>
      </ul>
      <template #footer>
        <button class="btn btn--primary btn--block" type="button" @click="showWarning = false">
          我已知晓，提高警惕
        </button>
      </template>
    </AppModal>

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

.topbar-link--primary {
  border-color: transparent;
  background: var(--primary);
  color: #fff;
  font-weight: 600;
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

/* ---------- 防骗提醒 ---------- */

.safety__lead {
  margin-bottom: 12px;
  font-weight: 600;
  line-height: 1.6;
  color: var(--danger);
}

.safety__list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding-left: 18px;
  font-size: 14px;
  line-height: 1.6;
}

/* 窄屏：顶栏收紧，用户胶囊只保留头像，给功能入口腾位置 */
@media (max-width: 480px) {
  .topbar {
    padding: 10px 12px;
  }

  .brand {
    font-size: 16px;
  }

  .topbar__right {
    gap: 6px;
  }

  .topbar-link {
    padding: 5px 8px;
  }

  .user-chip {
    gap: 0;
    padding: 3px;
  }

  .user-chip__name {
    display: none;
  }

  .stage {
    gap: 18px;
    padding: 18px 12px;
  }

  .stage__quota {
    text-align: center;
  }

  .stage__actions {
    gap: 12px;
  }

  .action {
    gap: 8px;
    padding: 22px 10px;
    border-radius: 16px;
  }

  .action__icon {
    font-size: 26px;
  }

  .action__label {
    font-size: 15px;
  }

  .strip {
    padding: 10px 0 14px;
  }

  .strip__list {
    gap: 8px;
    padding: 0 12px;
  }

  .bottle-card {
    width: 140px;
    height: 124px;
    padding: 10px;
  }
}
</style>
