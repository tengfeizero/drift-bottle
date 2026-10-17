<script setup>
import { computed, ref, watch } from 'vue'
import AppModal from './AppModal.vue'
import { useAuthStore } from '../stores/auth'
import { useBottleStore } from '../stores/bottles'
import { useToastStore } from '../stores/toast'
import { ErrorCode } from '../api/http'

const props = defineProps({
  open: { type: Boolean, default: false },
})

const emit = defineEmits(['update:open', 'close', 'picked', 'open-settings'])

const auth = useAuthStore()
const bottles = useBottleStore()
const toast = useToastStore()

// idle：待捞取；nomatch：海中无匹配；failed：其他失败
const phase = ref('idle')
const errorText = ref('')

const profileReady = computed(() => auth.profileComplete)

watch(
  () => props.open,
  (open) => {
    if (open) {
      phase.value = 'idle'
      errorText.value = ''
    }
  },
)

function close() {
  emit('update:open', false)
  emit('close')
}

async function pick() {
  phase.value = 'loading'
  errorText.value = ''
  try {
    const result = await bottles.pickBottle()
    toast.success('捞到了一个瓶子')
    close()
    emit('picked', result.id)
  } catch (error) {
    if (error.code === ErrorCode.NO_MATCH) {
      phase.value = 'nomatch'
    } else if (error.code === ErrorCode.PROFILE_INCOMPLETE) {
      phase.value = 'incomplete'
    } else if (error.code === ErrorCode.QUOTA_EXCEEDED) {
      phase.value = 'failed'
      errorText.value = '今日捞取次数已用完'
    } else {
      phase.value = 'failed'
      errorText.value = error.message || '捞取失败'
    }
  }
}
</script>

<template>
  <AppModal :open="open" title="捞瓶子" @close="close">
    <!-- 资料不全：引导先设置性别与年龄 -->
    <div v-if="!profileReady || phase === 'incomplete'" class="state">
      <p class="state__title">先完善你的资料</p>
      <p class="hint">
        只有设置了性别和年龄，才能被符合条件的瓶子匹配到。去「设置」补全后再来捞。
      </p>
      <button
        class="btn btn--primary btn--block state__action"
        type="button"
        @click="emit('open-settings'); close()"
      >
        去设置资料
      </button>
    </div>

    <!-- 无匹配 -->
    <div v-else-if="phase === 'nomatch'" class="state">
      <p class="state__title">海里暂时没有符合条件的瓶子</p>
      <p class="hint">本次没有消耗捞取次数，稍后再试或者自己扔一个吧。</p>
    </div>

    <!-- 其他失败 -->
    <div v-else-if="phase === 'failed'" class="state">
      <p class="state__title">捞取失败</p>
      <p class="hint">{{ errorText }}</p>
    </div>

    <!-- 待捞取 -->
    <div v-else class="state">
      <p class="state__title">把网撒进海里</p>
      <p class="hint">
        今日剩余捞取 <strong>{{ bottles.quotas.pick_remaining }}</strong> 次，捞到的瓶子会立刻打开。
      </p>
    </div>

    <template v-if="profileReady && phase !== 'incomplete'" #footer>
      <button
        class="btn btn--primary"
        type="button"
        :disabled="phase === 'loading'"
        @click="pick"
      >
        {{ phase === 'loading' ? '捞取中…' : phase === 'nomatch' || phase === 'failed' ? '再捞一次' : '捞一捞' }}
      </button>
    </template>
  </AppModal>
</template>

<style scoped>
.state {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.state__title {
  font-size: 16px;
  font-weight: 600;
}

.state__action {
  margin-top: 8px;
}
</style>
