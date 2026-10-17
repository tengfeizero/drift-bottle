<script setup>
import { computed } from 'vue'
import { defaultAvatars, defaultAvatarOf } from '../utils/avatar'

// 头像优先级：自定义上传图 → 用户选中的系统预设（1-12）→ 按 uid 哈希分配的默认生肖
const props = defineProps({
  src: { type: String, default: '' },
  avatarPreset: { type: Number, default: 0 },
  uid: { type: [String, Number], default: '' },
  name: { type: String, default: '' },
  size: { type: Number, default: 40 },
})

const initial = computed(() => (props.name || '漂').trim().slice(0, 1) || '漂')
const resolvedSrc = computed(() => {
  if (props.src) return props.src
  if (props.avatarPreset >= 1 && props.avatarPreset <= defaultAvatars.length) {
    return defaultAvatars[props.avatarPreset - 1]
  }
  return defaultAvatarOf(props.uid)
})
</script>

<template>
  <span
    class="avatar"
    :class="{ 'avatar--img': resolvedSrc }"
    :style="{ width: `${size}px`, height: `${size}px`, fontSize: `${Math.round(size * 0.42)}px` }"
  >
    <img v-if="resolvedSrc" :src="resolvedSrc" :alt="name" />
    <span v-else>{{ initial }}</span>
  </span>
</template>
