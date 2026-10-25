<template>
  <AppLayout>
    <div class="qx-chat">
      <p v-if="enabled === false" class="qx-chat-state" data-test="chat-disabled">{{ t('qxTheme.chat.disabled') }}</p>
      <div v-else-if="phase === 'error'" class="qx-chat-state" role="alert" data-test="chat-error">
        <p>{{ t('qxTheme.chat.sessionFailed') }}</p>
        <button type="button" class="btn btn-secondary btn-sm" data-test="chat-retry" @click="start">
          {{ t('qxTheme.chat.retry') }}
        </button>
      </div>
      <div v-else-if="phase === 'stale'" class="qx-chat-state" role="alert" data-test="chat-stale">
        <p>{{ t('qxTheme.chat.accountChanged') }}</p>
        <button type="button" class="btn btn-secondary btn-sm" @click="reloadPage">
          {{ t('qxTheme.chat.reload') }}
        </button>
      </div>
      <iframe
        v-else-if="phase === 'ready'"
        :key="frameKey"
        ref="frame"
        class="qx-chat-frame"
        :src="frameSrc"
        :title="t('qxTheme.chat.frameTitle')"
        allow="clipboard-read; clipboard-write; fullscreen"
        data-test="chat-frame"
        @load="sendSession"
      />
      <p v-else class="qx-chat-state" aria-live="polite" data-test="chat-loading">{{ t('qxTheme.chat.loading') }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
/**
 * CUSTOM: 对话工作台页面。整页只有一个同源 iframe（Open WebUI fork，经后端转发 /_chat/*）。
 *
 * - 加载 iframe 前先 POST /api/v1/chat/session 写入 HttpOnly Cookie；失败显示错误，不加载 iframe。
 * - 发现账号变化（同页换账号、本地数据记在别的用户名下）时先 DELETE（同时清 Open WebUI 自己的
 *   token Cookie）再 POST；普通首次加载与 token 刷新只 POST。
 * - iframe 地址带当前语言与深浅色（首屏预置）；iframe 发 chat:ready 后回 sub2api:chat-session；
 *   主题 / 语言变化时重发；token 刷新或 iframe 报 chat:session-expired 时重新签发 Cookie，revision + 1 再发。
 * - 退出登录由 auth store 调 revokeChatSession()（经 onChatRevoke 通知本页）；
 *   同页换账号、其他标签页换账号 / 退出时也先撤销，避免显示上一个用户的数据。
 * - 离开页面不撤销（iframe 随页面销毁，本地草稿保留给同一用户）。
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { AppLayout } from '@/components/layout'
import { isDocumentDark } from '@/composables/documentDark'
import {
  CHAT_REVOKE_MESSAGE,
  buildChatSession,
  chatFrameSrc,
  parseChatMessage,
  toChatLocale,
  type ChatRevokeMessage,
  type ChatSessionMessage
} from './chatBridge'
import {
  chatOwnedByOther,
  claimChatLocalData,
  clearChatLocalData,
  deleteChatSession,
  issueChatSession,
  onChatRevoke
} from './chatSession'
import { useChatEnabled } from './chatStatus'

type Phase = 'idle' | 'loading' | 'ready' | 'error' | 'stale'

const AUTH_TOKEN_KEY = 'auth_token'
const AUTH_USER_KEY = 'auth_user'

const { t, locale: i18nLocale } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const enabled = useChatEnabled()

const phase = ref<Phase>('idle')
const frame = ref<HTMLIFrameElement | null>(null)
const frameKey = ref(0)
const frameSrc = ref('')
const chatLocale = computed(() => toChatLocale(i18nLocale.value))

let revision = 0
/** 每次签发 / 撤销递增；回来的响应不是最新一次时丢弃，避免陈旧响应覆盖换账号后的状态。 */
let attempt = 0
/** 最近一次签发 Cookie 时所用的 access token（apiClient 从 localStorage 取）。 */
let issuedToken: string | null = null

function storedToken(): string | null {
  try {
    return localStorage.getItem(AUTH_TOKEN_KEY)
  } catch {
    return null
  }
}

function storedUserId(): number | null {
  try {
    const raw = localStorage.getItem(AUTH_USER_KEY)
    if (!raw) return null
    const id = Number((JSON.parse(raw) as { id?: unknown }).id)
    return Number.isFinite(id) && id > 0 ? id : null
  } catch {
    return null
  }
}

function post(message: ChatSessionMessage | ChatRevokeMessage) {
  frame.value?.contentWindow?.postMessage(message, window.location.origin)
}

function sendSession() {
  if (phase.value !== 'ready') return
  post(buildChatSession(revision, isDocumentDark(), chatLocale.value))
}

/** 签发 Cookie；reset 为 true（账号变化）时先清掉旧用户的两个 Cookie，DELETE 失败按签发失败处理。 */
async function issue(reset = false): Promise<'ok' | 'failed' | 'stale'> {
  const id = ++attempt
  const token = storedToken()
  try {
    if (reset) {
      await deleteChatSession()
      if (id !== attempt) return 'stale'
    }
    await issueChatSession()
  } catch {
    return id === attempt ? 'failed' : 'stale'
  }
  if (id !== attempt) return 'stale'
  issuedToken = token
  revision++
  return 'ok'
}

async function start() {
  const userId = authStore.user?.id
  if (enabled.value !== true || !authStore.token || userId === undefined) return
  phase.value = 'loading'
  // 归属标记在成功后才更新，DELETE 失败后重试仍会先 DELETE
  const result = await issue(chatOwnedByOther(userId))
  if (result === 'stale') return
  if (result === 'failed') {
    phase.value = 'error'
    return
  }
  claimChatLocalData(userId)
  frameSrc.value = chatFrameSrc(chatLocale.value, isDocumentDark() ? 'dark' : 'light')
  frameKey.value++
  phase.value = 'ready'
}

/** token 刷新 / Cookie 失效后重新签发；失败时保留 iframe，下次刷新 / 回到页面时再试。 */
async function refreshSession() {
  if (phase.value !== 'ready') return
  if ((await issue()) === 'ok') sendSession()
}

/** iframe 报 Cookie 失效：同一时间只重签一次，避免 iframe 连续报错时反复请求。 */
let expiredRefresh: Promise<void> | null = null
function onSessionExpired() {
  if (expiredRefresh) return
  expiredRefresh = refreshSession().finally(() => {
    expiredRefresh = null
  })
}

/** 撤销并卸载 iframe：先发 revoke，再清同源存储里的对话数据。 */
function revokeFrame(next: Phase = 'idle') {
  attempt++
  if (phase.value === 'ready') post(CHAT_REVOKE_MESSAGE)
  clearChatLocalData()
  phase.value = next
}

/** 其他标签页登录了别的账号或已退出：本页撤销并提示刷新；同一用户换了 token 则重新签发。 */
function syncWithStoredAuth() {
  if (phase.value !== 'ready' && phase.value !== 'loading') return
  const token = storedToken()
  if (!token || storedUserId() !== authStore.user?.id) {
    revokeFrame('stale')
    return
  }
  if (phase.value === 'ready' && token !== issuedToken) void refreshSession()
}

function onStorage(event: StorageEvent) {
  if (event.key === null || event.key === AUTH_TOKEN_KEY || event.key === AUTH_USER_KEY) syncWithStoredAuth()
}

function onVisibility() {
  if (document.visibilityState === 'visible') syncWithStoredAuth()
}

function onMessage(event: MessageEvent) {
  const msg = parseChatMessage(event, window.location.origin, frame.value?.contentWindow)
  if (!msg) return
  if (msg.type === 'chat:ready') sendSession()
  else if (msg.type === 'chat:balance-changed') authStore.refreshUser().catch(() => {})
  else if (msg.type === 'chat:navigate') void router.push(msg.path)
  else if (msg.type === 'chat:session-expired') onSessionExpired()
}

function reloadPage() {
  window.location.reload()
}

watch(
  enabled,
  (value) => {
    if (value === true && phase.value === 'idle') void start()
  },
  { immediate: true }
)

watch(
  () => [authStore.token, authStore.user?.id] as const,
  ([token, userId], [prevToken, prevUserId]) => {
    if (!token || userId === undefined) {
      revokeFrame()
      return
    }
    if (prevUserId !== undefined && userId !== prevUserId) {
      // 同页换账号：撤销后为新用户重新签发 Cookie 并重新加载 iframe（归属标记仍是旧用户 → 先 DELETE）
      revokeFrame()
      void start()
      return
    }
    if (token !== prevToken) void refreshSession()
  }
)

watch(
  () => [isDocumentDark(), chatLocale.value] as const,
  () => sendSession()
)

let unregisterRevoke: (() => void) | null = null

onMounted(() => {
  window.addEventListener('message', onMessage)
  window.addEventListener('storage', onStorage)
  document.addEventListener('visibilitychange', onVisibility)
  unregisterRevoke = onChatRevoke(() => revokeFrame())
})

onBeforeUnmount(() => {
  attempt++
  unregisterRevoke?.()
  window.removeEventListener('message', onMessage)
  window.removeEventListener('storage', onStorage)
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>

<style scoped>
/* 占满外壳内容区：视口高度减去顶栏（AppHeader h-16）；宽度随侧栏收起 / 移动端自适应。 */
.qx-chat {
  display: flex;
  height: calc(100dvh - 4rem);
  min-height: 0;
}
.qx-chat-frame {
  display: block;
  flex: 1;
  width: 100%;
  height: 100%;
  border: 0;
  background: transparent;
}
.qx-chat-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  margin: auto;
  color: var(--qx-muted, #6b7280);
  font-size: 0.875rem;
  text-align: center;
}
</style>
