<template>
  <AppLayout>
    <div class="qx-studio">
      <p v-if="enabled === false" class="qx-studio-empty">{{ t('qxTheme.studio.disabled') }}</p>
      <iframe
        v-else
        :key="frameKey"
        ref="frame"
        class="qx-studio-frame"
        :src="STUDIO_FRAME_SRC"
        :title="t('qxTheme.studio.frameTitle')"
        allow="clipboard-read; clipboard-write"
        @load="sendSession"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
/**
 * CUSTOM: 创作工作台页面。整页只有一个同源 iframe（Nova Studio，经后端转发 /_nova/*），
 * 父页只负责传会话（登录令牌、用户、角色、深浅色）；Key / 模型选择在 Nova 自己的设置页里。
 * 令牌刷新、角色或主题变化时重发会话；切换账号时先撤销再重新加载 iframe，退出或离开页面时撤销。
 */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { AppLayout } from '@/components/layout'
import { isDocumentDark } from '@/composables/documentDark'
import { STUDIO_FRAME_SRC, buildSession, parseNovaMessage } from './studioBridge'
import { useStudioEnabled } from './studioStatus'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const enabled = useStudioEnabled()

const frame = ref<HTMLIFrameElement | null>(null)
const frameKey = ref(0)
let revision = 0

function post(message: object) {
  frame.value?.contentWindow?.postMessage(message, window.location.origin)
}

function sendSession() {
  const session = buildSession(++revision, authStore.token, authStore.user, isDocumentDark())
  if (session) post(session)
}

function revoke() {
  post({ type: 'sub2api:nova-studio-revoke' })
}

function onMessage(event: MessageEvent) {
  const msg = parseNovaMessage(event, window.location.origin, frame.value?.contentWindow)
  if (!msg) return
  if (msg.type === 'nova:ready') sendSession()
  else if (msg.type === 'nova:balance-changed') authStore.refreshUser().catch(() => {})
  else if (msg.type === 'nova:navigate') router.push(msg.path)
}

watch(
  () => [authStore.token, authStore.user?.role, isDocumentDark()],
  () => sendSession()
)

watch(
  () => authStore.user?.id,
  (next, prev) => {
    if (prev === undefined || next === prev) return
    revoke()
    if (next !== undefined) frameKey.value++ // 换账号：重新加载，Nova 按新用户分区读取本地数据
  }
)

onMounted(() => window.addEventListener('message', onMessage))
onBeforeUnmount(() => {
  revoke()
  window.removeEventListener('message', onMessage)
})
</script>

<style scoped>
/* 占满外壳内容区：视口高度减去顶栏（AppHeader h-16）。 */
.qx-studio {
  display: flex;
  height: calc(100dvh - 4rem);
  min-height: 0;
}
.qx-studio-frame {
  display: block;
  flex: 1;
  width: 100%;
  height: 100%;
  border: 0;
  background: transparent;
}
.qx-studio-empty {
  margin: auto;
  color: var(--qx-muted, #6b7280);
  font-size: 0.875rem;
}
</style>
