<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import MediaAttachment from './MediaAttachment.vue'
import { useBottleStore } from '../stores/bottles'
import {
  ATTACHMENT_ACCEPT,
  MAX_ATTACHMENT_SIZE,
  attachmentKind,
  formatFileSize,
} from '../utils/attachment'

const props = defineProps({
  // 已上传完成的附件（含服务端 id 与 url）；null 表示未选择
  modelValue: { type: Object, default: null },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'update:uploading', 'error'])

const bottles = useBottleStore()

const inputRef = ref(null)
const uploading = ref(false)
// 选择文件后、上传完成前的本地预览
const localPreview = ref(null)

const preview = computed(() => props.modelValue || localPreview.value)

function revokeLocal() {
  if (localPreview.value) {
    URL.revokeObjectURL(localPreview.value.url)
    localPreview.value = null
  }
}

watch(
  () => props.modelValue,
  (value) => {
    if (value) revokeLocal()
  },
)

watch(uploading, (value) => emit('update:uploading', value))

onBeforeUnmount(revokeLocal)

function pick() {
  if (props.disabled || uploading.value) return
  inputRef.value?.click()
}

async function onChange(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return

  const kind = attachmentKind(file.name)
  if (!kind) {
    emit('error', '仅支持图片、音频与视频文件')
    return
  }
  if (file.size > MAX_ATTACHMENT_SIZE) {
    emit('error', `文件大小不能超过 50MB（当前 ${formatFileSize(file.size)}）`)
    return
  }

  revokeLocal()
  localPreview.value = { kind, url: URL.createObjectURL(file), name: file.name, size: file.size }
  uploading.value = true
  emit('error', '')
  try {
    const attachment = await bottles.uploadAttachment(file)
    emit('update:modelValue', attachment)
  } catch (error) {
    revokeLocal()
    emit('error', error.message || '附件上传失败')
  } finally {
    uploading.value = false
  }
}

function remove() {
  revokeLocal()
  emit('update:modelValue', null)
  emit('error', '')
}
</script>

<template>
  <div class="attach" :class="{ 'is-uploading': uploading }">
    <div v-if="preview" class="attach__card">
      <MediaAttachment :attachment="preview" show-meta />
      <button
        class="attach__remove"
        type="button"
        :disabled="disabled || uploading"
        @click="remove"
      >
        移除
      </button>
      <span v-if="uploading" class="attach__status">上传中…</span>
    </div>

    <div v-else class="attach__empty">
      <button
        class="btn btn--ghost attach__pick"
        type="button"
        :disabled="disabled || uploading"
        @click="pick"
      >
        添加图片 / 音频 / 视频
      </button>
      <span class="hint attach__hint">单个文件不超过 50MB</span>
    </div>

    <input
      ref="inputRef"
      class="attach__input"
      type="file"
      :accept="ATTACHMENT_ACCEPT"
      @change="onChange"
    />
  </div>
</template>

<style scoped>
.attach {
  display: flex;
  flex-direction: column;
}

.attach__card {
  position: relative;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
}

.attach.is-uploading .attach__card {
  opacity: 0.6;
}

/* 预览区限制媒体最大高度，避免过高撑破弹窗 */
.attach__card :deep(.media__image),
.attach__card :deep(.media__video) {
  max-height: 180px;
}

.attach__remove {
  position: absolute;
  top: 14px;
  right: 14px;
  padding: 3px 10px;
  font-size: 12px;
  color: #fff;
  background: rgba(0, 0, 0, 0.55);
  border: none;
  border-radius: 999px;
  cursor: pointer;
}

.attach__remove:disabled {
  cursor: not-allowed;
}

.attach__status {
  display: block;
  margin-top: 6px;
  font-size: 12px;
  color: var(--muted);
}

.attach__empty {
  display: flex;
  align-items: center;
  gap: 10px;
}

.attach__pick {
  padding: 8px 14px;
  font-size: 13px;
}

.attach__hint {
  font-size: 12px;
}

.attach__input {
  display: none;
}
</style>
