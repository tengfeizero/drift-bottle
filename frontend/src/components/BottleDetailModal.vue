<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import AvatarBadge from './AvatarBadge.vue'
import ConfirmDialog from './ConfirmDialog.vue'
import AttachmentField from './AttachmentField.vue'
import MediaAttachment from './MediaAttachment.vue'
import { useBottleStore, BottleState } from '../stores/bottles'
import { useToastStore } from '../stores/toast'
import { ErrorCode } from '../api/http'
import { formatTime, peerDescription } from '../utils/format'

const props = defineProps({
  open: { type: Boolean, default: false },
  bottleId: { type: Number, default: null },
})

const emit = defineEmits(['update:open', 'close', 'deleted'])

const bottles = useBottleStore()
const toast = useToastStore()

const detail = ref(null)
const loading = ref(false)
const sending = ref(false)
const draft = ref('')
const attachment = ref(null)
const attachUploading = ref(false)
const errorText = ref('')
const confirmDelete = ref(false)
const bodyRef = ref(null)

const peerName = computed(() => detail.value?.peer_nickname || '未知对方')
const peerDesc = computed(() => {
  const data = detail.value
  return data ? peerDescription(data.peer_gender, data.peer_age) : ''
})

const stateHint = computed(() => {
  const data = detail.value
  if (!data) return ''
  if (data.messages.length) return ''
  if (data.state === BottleState.Held) {
    return '回复后即开启一对一聊天；不回复也可以把瓶子扔回海里。'
  }
  return '还没有消息，打个招呼吧。'
})

// 捞取者视角：HELD 状态只有捞取者能进入详情，因此可视为持有者
const canReturn = computed(() => detail.value?.state === BottleState.Held)

function close() {
  emit('update:open', false)
  emit('close')
}

function reportError(error) {
  if (error.code === ErrorCode.BOTTLE_NOT_FOUND) {
    toast.error('瓶子不存在或已被删除')
    close()
    return
  }
  errorText.value = error.message || '操作失败'
}

async function scrollToBottom() {
  await nextTick()
  if (bodyRef.value) bodyRef.value.scrollTop = bodyRef.value.scrollHeight
}

async function load() {
  if (!props.bottleId) return
  loading.value = true
  errorText.value = ''
  try {
    detail.value = await bottles.fetchDetail(props.bottleId)
    scrollToBottom()
  } catch (error) {
    reportError(error)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      detail.value = null
      draft.value = ''
      attachment.value = null
      attachUploading.value = false
      errorText.value = ''
      load()
    }
  },
)

watch(
  () => props.bottleId,
  () => {
    if (props.open) {
      detail.value = null
      attachment.value = null
      load()
    }
  },
)

// 实时联动：对方新消息 / 状态变化时刷新会话；瓶子被删除则关闭窗口
watch(
  () => bottles.eventSignal,
  (signal) => {
    if (!props.open || !signal || signal.bottleId !== props.bottleId) return
    if (signal.type === 'bottle_deleted') {
      toast.error('对方已删除该瓶子')
      close()
      emit('deleted', props.bottleId)
      return
    }
    load()
  },
)

async function send() {
  const content = draft.value.trim()
  // 文字可为空，但至少要有一个附件
  if (!content && !attachment.value) return
  if (content.length > 500) {
    errorText.value = '消息最多 500 字'
    return
  }

  sending.value = true
  errorText.value = ''
  try {
    const message = await bottles.sendMessage(props.bottleId, content, attachment.value?.id || 0)
    if (detail.value) {
      detail.value.messages.push(message)
      if (detail.value.state === BottleState.Held) {
        detail.value.state = BottleState.Chatting
      }
    }
    draft.value = ''
    attachment.value = null
    scrollToBottom()
  } catch (error) {
    reportError(error)
  } finally {
    sending.value = false
  }
}

async function returnToSea() {
  try {
    await bottles.returnBottle(props.bottleId)
    toast.success('已扔回海里')
    close()
    emit('deleted', props.bottleId)
  } catch (error) {
    reportError(error)
  }
}

async function remove() {
  confirmDelete.value = false
  try {
    await bottles.deleteBottle(props.bottleId)
    toast.success('瓶子已删除')
    close()
    emit('deleted', props.bottleId)
  } catch (error) {
    reportError(error)
  }
}
</script>

<template>
  <Transition name="modal">
    <div v-if="open" class="modal-mask" @click.self="close">
      <div class="modal-card chat-card">
        <header class="chat-head">
          <AvatarBadge :src="detail?.peer_avatar" :uid="detail?.peer_id" :name="peerName" :size="40" />
          <div class="chat-head__meta">
            <strong>{{ peerName }}</strong>
            <span class="hint">{{ peerDesc }}</span>
          </div>
          <button
            v-if="canReturn"
            class="btn btn--ghost chat-head__action"
            type="button"
            @click="returnToSea"
          >
            扔回海里
          </button>
          <button
            class="icon-btn"
            type="button"
            aria-label="删除瓶子"
            @click="confirmDelete = true"
          >
            🗑
          </button>
          <button class="icon-btn" type="button" aria-label="关闭" @click="close">×</button>
        </header>

        <div ref="bodyRef" class="chat-body">
          <p v-if="loading && !detail" class="hint chat-tip">加载中…</p>

          <template v-else-if="detail">
            <div class="bubble-row">
              <div class="bubble">
                <MediaAttachment v-if="detail.attachment" :attachment="detail.attachment" />
                <span v-if="detail.content" class="bubble__text">{{ detail.content }}</span>
                <em class="bubble__time">{{ formatTime(detail.created_at) }}</em>
              </div>
            </div>

            <p v-if="stateHint" class="hint chat-tip">{{ stateHint }}</p>

            <div
              v-for="message in detail.messages || []"
              :key="message.id"
              class="bubble-row"
              :class="{ 'is-mine': message.mine }"
            >
              <div class="bubble">
                <MediaAttachment v-if="message.attachment" :attachment="message.attachment" />
                <span v-if="message.content" class="bubble__text">{{ message.content }}</span>
                <em class="bubble__time">{{ formatTime(message.created_at) }}</em>
              </div>
            </div>
          </template>
        </div>

        <p v-if="errorText" class="error-text chat-error">{{ errorText }}</p>

        <div class="chat-composer">
          <AttachmentField
            v-model="attachment"
            :disabled="sending"
            @update:uploading="attachUploading = $event"
            @error="errorText = $event"
          />
          <div class="chat-composer__row">
            <input
              v-model="draft"
              class="input"
              maxlength="500"
              placeholder="说点什么…"
              @keyup.enter="send"
            />
            <button
              class="btn btn--primary"
              type="button"
              :disabled="sending || attachUploading || (!draft.trim() && !attachment)"
              @click="send"
            >
              {{ sending ? '发送中' : '发送' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </Transition>

  <ConfirmDialog
    :open="confirmDelete"
    title="删除瓶子"
    message="删除后瓶子与全部聊天记录将对双方彻底移除，不可恢复。确定要删除吗？"
    confirm-text="删除"
    danger
    @confirm="remove"
    @cancel="confirmDelete = false"
  />
</template>

<style scoped>
.chat-card {
  max-width: 768px;
  height: 82svh;
}

.chat-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px 10px;
  border-bottom: 1px solid var(--border);
}

.chat-head__meta {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.chat-head__meta strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chat-head__action {
  padding: 6px 12px;
  font-size: 13px;
}

.chat-body {
  flex: 1;
  overflow-y: auto;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: #ededed;
}

.chat-tip {
  text-align: center;
  padding: 20px 8px;
}

.bubble-row {
  display: flex;
}

.bubble-row.is-mine {
  justify-content: flex-end;
}

.bubble {
  position: relative;
  max-width: 76%;
  padding: 8px 12px;
  border-radius: 4px;
  background: #fff;
  color: #191919;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

/* 微信式气泡小尖角：对方朝左，自己朝右 */
.bubble::before {
  content: '';
  position: absolute;
  top: 13px;
  left: -5px;
  width: 0;
  height: 0;
  border-style: solid;
  border-width: 5px 6px 5px 0;
  border-color: transparent #fff transparent transparent;
}

.bubble-row.is-mine .bubble {
  background: #95ec69;
  color: #191919;
}

.bubble-row.is-mine .bubble::before {
  left: auto;
  right: -5px;
  border-width: 5px 0 5px 6px;
  border-color: transparent transparent transparent #95ec69;
}

.bubble__text {
  font-size: 16px;
  line-height: 1.5;
  word-break: break-word;
  white-space: pre-wrap;
}

.bubble__time {
  font-size: 12px;
  opacity: 0.7;
  font-style: normal;
  align-self: flex-end;
}

.chat-error {
  padding: 0 16px;
}

.chat-composer {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 16px 14px;
  border-top: 1px solid var(--border);
}

.chat-composer__row {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.chat-composer__row .input {
  width: 100%;
}
</style>
