<script setup>
import { useToastStore } from './stores/toast'

// 应用根组件：路由出口 + 全局轻提示容器
const toast = useToastStore()
</script>

<template>
  <router-view />

  <div class="toast-host">
    <TransitionGroup name="toast">
      <div
        v-for="item in toast.items"
        :key="item.id"
        class="toast"
        :class="`toast--${item.type}`"
      >
        {{ item.message }}
      </div>
    </TransitionGroup>
  </div>
</template>
