<script setup>
import { ref, watch } from 'vue'
import AppModal from './AppModal.vue'
import AvatarBadge from './AvatarBadge.vue'
import { defaultAvatars } from '../utils/avatar'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import { ErrorCode } from '../api/http'

const props = defineProps({
  open: { type: Boolean, default: false },
})

const emit = defineEmits(['update:open', 'close'])

const auth = useAuthStore()
const toast = useToastStore()

const GENDERS = [
  { value: 0, label: '未设置' },
  { value: 1, label: '男' },
  { value: 2, label: '女' },
]

const nickname = ref('')
const gender = ref(0)
const age = ref(18)
const saving = ref(false)
const uploading = ref(false)
const presetSaving = ref(false)
const errorText = ref('')
const fileInput = ref(null)

watch(
  () => props.open,
  (open) => {
    if (open) {
      nickname.value = auth.nickname
      gender.value = auth.gender
      age.value = auth.age || 18
      errorText.value = ''
    }
  },
)

function close() {
  if (saving.value || presetSaving.value || uploading.value) return
  emit('update:open', false)
  emit('close')
}

async function save() {
  const name = nickname.value.trim()
  if (name.length < 1 || name.length > 20) {
    errorText.value = '昵称需为 1-20 个字符'
    return
  }
  const parsedAge = Number(age.value) || 0
  if (parsedAge < 0 || parsedAge > 150) {
    errorText.value = '年龄需为 0-150'
    return
  }

  saving.value = true
  errorText.value = ''
  try {
    await auth.updateProfile({ nickname: name, gender: gender.value, age: parsedAge })
    toast.success('资料已更新')
  } catch (error) {
    errorText.value = error.message || '保存失败'
  } finally {
    saving.value = false
  }
  // close() 在保存中会被拦截，因此先解除 saving 再关窗
  if (!errorText.value) close()
}

function chooseAvatar() {
  fileInput.value?.click()
}

async function onFileChange(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return
  if (file.size > 2 * 1024 * 1024) {
    errorText.value = '头像仅支持不超过 2MB 的 JPG、PNG、WebP 图片'
    return
  }

  uploading.value = true
  errorText.value = ''
  try {
    await auth.uploadAvatar(file)
    toast.success('头像已更新')
  } catch (error) {
    errorText.value = error.code === ErrorCode.AVATAR_INVALID ? '头像仅支持 JPG、PNG、WebP 且不超过 2MB' : error.message
  } finally {
    uploading.value = false
  }
}

async function choosePreset(idx) {
  const preset = idx + 1
  if (auth.avatarPreset === preset && !auth.avatar) return // 已是该预设
  presetSaving.value = true
  errorText.value = ''
  try {
    await auth.setAvatarPreset(preset)
    toast.success('头像已更新')
  } catch (error) {
    errorText.value = error.message || '设置预设头像失败'
  } finally {
    presetSaving.value = false
  }
}

async function restoreDefault() {
  if (auth.avatarPreset === 0 && !auth.avatar) return // 已经是系统默认
  presetSaving.value = true
  errorText.value = ''
  try {
    await auth.setAvatarPreset(0)
    toast.success('已恢复系统默认头像')
  } catch (error) {
    errorText.value = error.message || '恢复默认失败'
  } finally {
    presetSaving.value = false
  }
}

// 判断当前某个预设是否为激活态：preset 匹配且没有上传图覆盖
function isPresetActive(idx) {
  return auth.avatarPreset === idx + 1 && !auth.avatar
}
</script>

<template>
  <AppModal :open="open" title="设置" :closable="!saving && !presetSaving && !uploading" @close="close">
    <div class="avatar-section">
      <div class="avatar-preview">
        <AvatarBadge :src="auth.avatar" :avatar-preset="auth.avatarPreset" :uid="auth.id" :name="auth.nickname" :size="72" />
      </div>
      <div class="avatar-actions">
        <button class="btn" type="button" :disabled="uploading || presetSaving" @click="chooseAvatar">
          {{ uploading ? '上传中…' : '上传图片' }}
        </button>
        <button
          class="btn btn--ghost"
          type="button"
          :disabled="presetSaving || (auth.avatarPreset === 0 && !auth.avatar)"
          @click="restoreDefault"
        >
          {{ presetSaving ? '保存中…' : '恢复系统默认' }}
        </button>
      </div>
      <input
        ref="fileInput"
        class="file-input"
        type="file"
        accept="image/jpeg,image/png,image/webp"
        @change="onFileChange"
      />

      <div class="preset-grid">
        <button
          v-for="(src, idx) in defaultAvatars"
          :key="idx"
          type="button"
          class="preset-item"
          :class="{ 'is-active': isPresetActive(idx) }"
          :disabled="presetSaving"
          @click="choosePreset(idx)"
        >
          <img :src="src" :alt="`默认头像 ${idx + 1}`" />
        </button>
      </div>
    </div>

    <div class="field">
      <input
        id="nickname"
        v-model="nickname"
        class="input"
        maxlength="20"
        placeholder="1-20 个字符"
      />
    </div>

    <div class="field">
      <div class="radio-group">
        <button
          v-for="item in GENDERS"
          :key="item.value"
          class="radio-chip"
          :class="{ 'is-active': gender === item.value }"
          type="button"
          @click="gender = item.value"
        >
          {{ item.label }}
        </button>
      </div>
    </div>

    <div class="field age-field">
      <input id="age" v-model="age" class="input" type="number" min="18" max="96" />
      <span class="age-unit">岁</span>
    </div>

    <p v-if="errorText" class="error-text">{{ errorText }}</p>

    <div class="account">
      <span class="hint">我的 ID：<strong>{{ auth.id }}</strong></span>
      <p class="hint">ID 是登录的唯一凭证，请务必牢记。</p>
    </div>

    <template #footer>
      <button class="btn btn--primary" type="button" :disabled="saving" @click="save">
        {{ saving ? '保存中…' : '保存资料' }}
      </button>
    </template>
  </AppModal>
</template>

<style scoped>
.avatar-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.avatar-preview {
  display: flex;
  justify-content: center;
}

.avatar-actions {
  display: flex;
  gap: 10px;
}

.file-input {
  display: none;
}

.preset-label {
  font-size: 13px;
  color: var(--muted);
  margin: 16px 0 6px;
}

.preset-grid {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 10px;
  width: 100%;
}

.preset-item {
  background: transparent;
  border: 2px solid transparent;
  border-radius: 50%;
  padding: 2px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: border-color 0.15s ease;
}

.preset-item img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
  display: block;
}

.preset-item:hover {
  border-color: var(--primary);
}

.preset-item.is-active {
  border-color: var(--primary);
}

.preset-item:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.account {
  margin-top: 6px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--surface-2);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.age-field {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 6px;
}

.age-field .input {
  width: auto;
  flex: 1;
  min-width: 0;
}

.age-unit {
  color: var(--primary);
  white-space: nowrap;
  flex-shrink: 0;
}
</style>
