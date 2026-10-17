<script setup>
import { ref } from 'vue'
import AppModal from './AppModal.vue'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

defineProps({
  open: { type: Boolean, default: false },
})

const emit = defineEmits(['update:open', 'close'])

const auth = useAuthStore()
const toast = useToastStore()

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const submitting = ref(false)
const errorText = ref('')

function close() {
  if (submitting.value) return
  emit('update:open', false)
  emit('close')
}

function reset() {
  oldPassword.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  errorText.value = ''
}

function validate() {
  if (!oldPassword.value) return '请输入原密码'
  if (newPassword.value.length < 6 || newPassword.value.length > 32) {
    return '新密码需为 6-32 位'
  }
  if (!/[A-Za-z]/.test(newPassword.value) || !/\d/.test(newPassword.value)) {
    return '新密码需同时包含字母与数字'
  }
  if (newPassword.value !== confirmPassword.value) return '两次输入的新密码不一致'
  return ''
}

async function submit() {
  errorText.value = validate()
  if (errorText.value) return

  submitting.value = true
  try {
    await auth.changePassword(oldPassword.value, newPassword.value)
    toast.success('密码已修改')
    reset()
    close()
  } catch (error) {
    errorText.value = error.message || '修改失败'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <AppModal
    :open="open"
    title="修改密码"
    :closable="!submitting"
    @close="close"
  >
    <div class="field">
      <input
        id="old-password"
        v-model="oldPassword"
        class="input"
        type="password"
        autocomplete="current-password"
        placeholder="请输入原密码"
      />
    </div>

    <div class="field">
      <input
        id="new-password"
        v-model="newPassword"
        class="input"
        type="password"
        autocomplete="new-password"
        placeholder="6-32 位，需含字母与数字"
      />
    </div>

    <div class="field">
      <input
        id="confirm-password"
        v-model="confirmPassword"
        class="input"
        type="password"
        autocomplete="new-password"
        placeholder="再次输入新密码"
        @keyup.enter="submit"
      />
    </div>

    <p v-if="errorText" class="error-text">{{ errorText }}</p>

    <template #footer>
      <button class="btn btn--primary" type="button" :disabled="submitting" @click="submit">
        {{ submitting ? '提交中…' : '确认修改' }}
      </button>
    </template>
  </AppModal>
</template>
