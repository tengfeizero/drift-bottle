<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const mode = ref('login')
const id = ref('')
const password = ref('')
const confirmPassword = ref('')
const submitting = ref(false)
const errorText = ref('')
// 注册成功后先展示 ID，用户确认「已记住」再进入主界面
const registeredId = ref('')

const isRegister = computed(() => mode.value === 'register')

function switchMode(next) {
  mode.value = next
  errorText.value = ''
  password.value = ''
  confirmPassword.value = ''
}

function validate() {
  if (!isRegister.value && !/^\d{8}$/.test(id.value.trim())) {
    return '请输入 8 位数字 ID'
  }
  if (password.value.length < 6 || password.value.length > 32) {
    return '密码需为 6-32 位'
  }
  if (!/[A-Za-z]/.test(password.value) || !/\d/.test(password.value)) {
    return '密码需同时包含字母与数字'
  }
  if (isRegister.value && password.value !== confirmPassword.value) {
    return '两次输入的密码不一致'
  }
  return ''
}

async function submit() {
  errorText.value = validate()
  if (errorText.value) return

  submitting.value = true
  try {
    if (isRegister.value) {
      const data = await auth.register(password.value)
      registeredId.value = data.id
    } else {
      await auth.login(id.value.trim(), password.value)
      toast.success('登录成功')
      router.replace({ name: 'home' })
    }
  } catch (error) {
    errorText.value = error.message || '操作失败'
  } finally {
    submitting.value = false
  }
}

async function copyId() {
  try {
    await navigator.clipboard.writeText(registeredId.value)
    toast.success('ID 已复制')
  } catch {
    toast.error('复制失败，请手动记录')
  }
}

function enterHome() {
  toast.success('欢迎加入漂流瓶')
  router.replace({ name: 'home' })
}
</script>

<template>
  <div class="auth">
    <div class="auth__card">
      <h1 class="auth__title">漂流瓶</h1>
      <p class="auth__subtitle">扔出心事，捞起缘分</p>

      <!-- 注册成功：展示 ID 并要求确认 -->
      <div v-if="registeredId" class="issued">
        <p class="issued__tip">注册成功，这是你的专属 ID</p>
        <p class="issued__id">{{ registeredId }}</p>
        <p class="hint">
          请务必牢记。ID 是登录的唯一凭证，忘记后无法找回。
        </p>
        <button class="btn btn--ghost btn--block issued__copy" type="button" @click="copyId">
          复制 ID
        </button>
        <button class="btn btn--primary btn--block" type="button" @click="enterHome">
          我已记住，进入漂流瓶
        </button>
      </div>

      <!-- 登录 / 注册表单 -->
      <div v-else>
        <div class="tabs">
          <button
            class="tabs__item"
            :class="{ 'is-active': !isRegister }"
            type="button"
            @click="switchMode('login')"
          >
            登录
          </button>
          <button
            class="tabs__item"
            :class="{ 'is-active': isRegister }"
            type="button"
            @click="switchMode('register')"
          >
            注册
          </button>
        </div>

        <div v-if="!isRegister" class="field">
          <input
            id="login-id"
            v-model="id"
            class="input"
            inputmode="numeric"
            maxlength="8"
            placeholder="8 位数字 ID"
          />
        </div>

        <div class="field">
          <input
            id="login-password"
            v-model="password"
            class="input"
            type="password"
            :autocomplete="isRegister ? 'new-password' : 'current-password'"
            placeholder="6-32 位，需含字母与数字"
          />
        </div>

        <div v-if="isRegister" class="field">
          <input
            id="login-confirm"
            v-model="confirmPassword"
            class="input"
            type="password"
            autocomplete="new-password"
            placeholder="再次输入密码"
          />
        </div>

        <p v-if="errorText" class="error-text auth__error">{{ errorText }}</p>

        <button
          class="btn btn--primary btn--lg btn--block"
          type="button"
          :disabled="submitting"
          @click="submit"
        >
          {{ submitting ? '请稍候…' : isRegister ? '注册并获取 ID' : '登录' }}
        </button>

        <p class="hint auth__foot">
          {{ isRegister ? '注册只需设置一个密码，系统会为你分配 8 位数字 ID。' : '忘记 ID 将无法找回，请妥善保管。' }}
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.auth {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
  background: linear-gradient(160deg, #0ea5e9 0%, #38bdf8 45%, #eef4f9 100%);
}

.auth__card {
  width: 100%;
  max-width: 380px;
  padding: 26px 22px;
  border-radius: 18px;
  background: var(--surface);
  box-shadow: 0 14px 40px rgba(15, 42, 71, 0.18);
}

.auth__title {
  font-size: 26px;
  text-align: center;
}

.auth__subtitle {
  margin-top: 6px;
  margin-bottom: 20px;
  font-size: 13px;
  text-align: center;
  color: var(--muted);
}

.tabs {
  display: flex;
  gap: 6px;
  padding: 4px;
  margin-bottom: 18px;
  border-radius: 999px;
  background: var(--surface-2);
}

.tabs__item {
  flex: 1;
  padding: 8px;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: var(--muted);
}

.tabs__item.is-active {
  background: var(--surface);
  color: var(--primary-dark);
  font-weight: 600;
}

.auth__error {
  margin-bottom: 12px;
}

.auth__foot {
  margin-top: 14px;
  text-align: center;
  line-height: 1.6;
}

.issued {
  display: flex;
  flex-direction: column;
  gap: 10px;
  text-align: center;
}

.issued__tip {
  font-size: 14px;
  color: var(--muted);
}

.issued__id {
  font-size: 34px;
  font-weight: 700;
  letter-spacing: 4px;
  color: var(--primary-dark);
}

.issued__copy {
  margin-top: 4px;
}
</style>
