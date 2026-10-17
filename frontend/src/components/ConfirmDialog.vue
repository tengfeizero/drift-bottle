<script setup>
import AppModal from './AppModal.vue'

// 二次确认框：删除瓶子、退出登录等不可逆操作
defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '请确认' },
  message: { type: String, default: '' },
  confirmText: { type: String, default: '确定' },
  danger: { type: Boolean, default: false },
  busy: { type: Boolean, default: false },
})

const emit = defineEmits(['confirm', 'cancel'])
</script>

<template>
  <AppModal :open="open" :title="title" @close="emit('cancel')">
    <p class="confirm-message">{{ message }}</p>

    <template #footer>
      <button
        class="btn"
        :class="danger ? 'btn--danger' : 'btn--primary'"
        type="button"
        :disabled="busy"
        @click="emit('confirm')"
      >
        {{ confirmText }}
      </button>
    </template>
  </AppModal>
</template>

<style scoped>
.confirm-message {
  font-size: 15px;
  line-height: 1.6;
  color: var(--text);
}
</style>
