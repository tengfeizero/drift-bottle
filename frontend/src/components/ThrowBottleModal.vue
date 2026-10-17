<script setup>
import { ref, watch } from 'vue'
import AppModal from './AppModal.vue'
import AttachmentField from './AttachmentField.vue'
import { useBottleStore } from '../stores/bottles'
import { useToastStore } from '../stores/toast'
import { ErrorCode } from '../api/http'

const props = defineProps({
  open: { type: Boolean, default: false },
})

const emit = defineEmits(['update:open', 'close', 'thrown'])

const bottles = useBottleStore()
const toast = useToastStore()

const GENDERS = [
  { value: 0, label: '不限' },
  { value: 1, label: '男' },
  { value: 2, label: '女' },
]

const content = ref('')
const attachment = ref(null)
const attachUploading = ref(false)
const targetGender = ref(0)
const ageMin = ref(18)
const ageMax = ref(99)
const submitting = ref(false)
const errorText = ref('')

function resetForm() {
  content.value = ''
  attachment.value = null
  attachUploading.value = false
  targetGender.value = 0
  ageMin.value = 18
  ageMax.value = 96
  errorText.value = ''
}

watch(
  () => props.open,
  (open) => {
    if (open) resetForm()
  },
)

function close() {
  if (submitting.value) return
  emit('update:open', false)
  emit('close')
}

function validate() {
  const text = content.value.trim()
  if (text.length > 500) return '投放内容最多 500 字'
  if (!text && !attachment.value) return '写点什么，或添加一个附件'
  const min = Number(ageMin.value)
  const max = Number(ageMax.value)
  if (!Number.isInteger(min) || !Number.isInteger(max) || min < 18 || max > 96) {
    return '目标年龄范围需为 18-96'
  }
  if (min > max) return '年龄下限不能大于上限'
  return ''
}

async function submit() {
  errorText.value = validate()
  if (errorText.value) return

  submitting.value = true
  try {
    const bottle = await bottles.throwBottle({
      content: content.value.trim(),
      attachment_id: attachment.value?.id || 0,
      target_gender: targetGender.value,
      target_age_min: Number(ageMin.value),
      target_age_max: Number(ageMax.value),
    })
    toast.success('瓶子已投进海里')
    emit('thrown', bottle.id)
    resetForm()
  } catch (error) {
    if (error.code === ErrorCode.QUOTA_EXCEEDED) {
      errorText.value = '今日投放次数已用完'
    } else {
      errorText.value = error.message || '投放失败'
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <AppModal :open="open" title="扔瓶子" :closable="!submitting" @close="close">
    <p class="quota-hint">
      今日剩余投放
      <strong>{{ bottles.quotas.throw_remaining }}</strong> 次
    </p>

    <div class="field">
      <textarea
        id="bottle-content"
        v-model="content"
        class="textarea"
        maxlength="500"
        placeholder="写下你想说的话，让它漂向有缘人…"
      />
      <span class="counter">{{ content.length }} / 500</span>
    </div>

    <div class="field">
      <AttachmentField
        v-model="attachment"
        :disabled="submitting"
        @update:uploading="attachUploading = $event"
        @error="errorText = $event"
      />
    </div>

    <div class="field">
      <div class="radio-group">
        <button
          v-for="item in GENDERS"
          :key="item.value"
          class="radio-chip"
          :class="{ 'is-active': targetGender === item.value }"
          type="button"
          @click="targetGender = item.value"
        >
          {{ item.label }}
        </button>
      </div>
    </div>

    <div class="field">
      <div class="age-row">
        <input v-model="ageMin" class="input" type="number" min="18" max="96" placeholder="最小" />
        <span class="age-unit">岁</span>
        <span class="age-sep">至</span>
        <input v-model="ageMax" class="input" type="number" min="18" max="96" placeholder="最大" />
        <span class="age-unit">岁</span>
      </div>
    </div>

    <p v-if="errorText" class="error-text">{{ errorText }}</p>

    <template #footer>
      <button
        class="btn btn--primary"
        type="button"
        :disabled="submitting || attachUploading"
        @click="submit"
      >
        {{ submitting ? '投放中…' : '扔进海里' }}
      </button>
    </template>
  </AppModal>
</template>

<style scoped>
.quota-hint {
  margin-bottom: 16px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--primary-soft);
  text-align: center;
  font-size: 14px;
  color: var(--primary-dark);
}

.quota-hint strong {
  margin: 0 2px;
  font-size: 20px;
  font-weight: 700;
  color: var(--primary-dark);
}

.age-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.age-row .input {
  width: auto;
  flex: 1;
  min-width: 0;
}

.age-sep {
  color: var(--primary);
  white-space: nowrap;
  flex-shrink: 0;
}

.age-unit {
  color: var(--primary);
  white-space: nowrap;
  flex-shrink: 0;
}
</style>
