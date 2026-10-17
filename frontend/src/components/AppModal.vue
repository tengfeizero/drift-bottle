<script setup>
// 通用弹窗外壳：遮罩点击关闭 + 可选标题 / 头部插槽 / 底部插槽
defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '' },
  closable: { type: Boolean, default: true },
})

const emit = defineEmits(['close'])
</script>

<template>
  <Transition name="modal">
    <div v-if="open" class="modal-mask" @click.self="closable && emit('close')">
      <div class="modal-card">
        <header v-if="title || $slots.head" class="modal-head">
          <slot name="head">
            <h3>{{ title }}</h3>
          </slot>
          <button
            v-if="closable"
            class="icon-btn"
            type="button"
            aria-label="关闭"
            @click="emit('close')"
          >
            ×
          </button>
        </header>

        <div class="modal-body">
          <slot />
        </div>

        <footer v-if="$slots.footer" class="modal-foot">
          <slot name="footer" />
        </footer>
      </div>
    </div>
  </Transition>
</template>
