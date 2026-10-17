<script setup>
import { attachmentLabel, formatFileSize } from '../utils/attachment'

defineProps({
  attachment: { type: Object, required: true },
  // 是否展示文件名与大小（用于上传预览，聊天气泡内不显示）
  showMeta: { type: Boolean, default: false },
})
</script>

<template>
  <div v-if="attachment" class="media" :class="`is-${attachment.kind}`">
    <a
      v-if="attachment.kind === 'image'"
      class="media__box"
      :href="attachment.url"
      target="_blank"
      rel="noopener"
    >
      <img class="media__image" :src="attachment.url" :alt="attachment.name || '图片'" loading="lazy" />
    </a>
    <audio
      v-else-if="attachment.kind === 'audio'"
      class="media__audio"
      :src="attachment.url"
      controls
      preload="metadata"
    />
    <video
      v-else-if="attachment.kind === 'video'"
      class="media__video"
      :src="attachment.url"
      controls
      preload="metadata"
      playsinline
    />
    <span v-else class="media__fallback">{{ attachmentLabel(attachment.kind) }}</span>

    <p v-if="showMeta" class="media__meta">
      <span class="media__name">{{ attachment.name }}</span>
      <span>{{ formatFileSize(attachment.size) }}</span>
    </p>
  </div>
</template>

<style scoped>
.media {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.media__box {
  display: block;
  line-height: 0;
  border-radius: 10px;
  overflow: hidden;
  background: var(--surface-2);
}

.media__image {
  display: block;
  max-width: 100%;
  max-height: 240px;
  object-fit: contain;
}

.media__audio {
  /* 固定宽度，避免在 fit-content 气泡内被压缩成不可用的窄条 */
  width: 260px;
  max-width: 100%;
  height: 36px;
}

.media__video {
  display: block;
  width: 100%;
  max-height: 260px;
  border-radius: 10px;
  background: #000;
}

.media__fallback {
  font-size: 14px;
}

.media__meta {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  font-size: 12px;
  opacity: 0.75;
}

.media__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
