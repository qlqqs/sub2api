/**
 * CUSTOM: 对话工作台（Open WebUI fork，经后端转发 /_chat/*）父页与 iframe 之间的 postMessage 契约。
 * 完整约定见 .trellis/tasks/10-24-chat-openwebui/contract.md E 节；
 * iframe 侧实现在 fork 的 src/lib/embed/bridge.ts。
 *
 * 与创作工作台不同，这里不经 postMessage 传任何令牌：身份由 POST /api/v1/chat/session
 * 写入的 HttpOnly Cookie（Path=/_chat）携带，revision 只用于调试。
 */

export type ChatLocale = 'zh-CN' | 'en-US'
export type ChatTheme = 'light' | 'dark'

export interface ChatSessionMessage {
  type: 'sub2api:chat-session'
  revision: number
  theme: ChatTheme
  locale: ChatLocale
}

export interface ChatRevokeMessage {
  type: 'sub2api:chat-revoke'
}

/** iframe 只能请求跳到这些站内页面（余额不足「去充值」、Key 失效「去管理 Key」）。 */
export const CHAT_NAVIGATE_ALLOWLIST = ['/purchase', '/keys'] as const
export type ChatNavigatePath = (typeof CHAT_NAVIGATE_ALLOWLIST)[number]

export type ChatFrameMessage =
  | { type: 'chat:ready' }
  | { type: 'chat:balance-changed' }
  | { type: 'chat:navigate'; path: ChatNavigatePath }
  | { type: 'chat:session-expired' }

export const CHAT_REVOKE_MESSAGE: ChatRevokeMessage = { type: 'sub2api:chat-revoke' }

/** 控制台语言（vue-i18n 的 zh / en）→ iframe 语言。 */
export function toChatLocale(locale: string | null | undefined): ChatLocale {
  return typeof locale === 'string' && locale.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US'
}

/**
 * iframe 地址：只在首次加载（及换账号重载）时取当前语言与深浅色，供 iframe 首屏预置、避免闪烁；
 * 之后的变化经 chat-session 下发，不重载 iframe。
 */
export function chatFrameSrc(locale: ChatLocale, theme: ChatTheme): string {
  return `/_chat/?lang=${encodeURIComponent(locale)}&theme=${encodeURIComponent(theme)}`
}

export function buildChatSession(revision: number, dark: boolean, locale: ChatLocale): ChatSessionMessage {
  return { type: 'sub2api:chat-session', revision, theme: dark ? 'dark' : 'light', locale }
}

export function isAllowedChatNavigatePath(path: unknown): path is ChatNavigatePath {
  return typeof path === 'string' && (CHAT_NAVIGATE_ALLOWLIST as readonly string[]).includes(path)
}

/** 只接受同源、且来自对话 iframe 的消息；其余一律忽略。 */
export function parseChatMessage(
  event: Pick<MessageEvent, 'origin' | 'source' | 'data'>,
  expectedOrigin: string,
  frameWindow: unknown
): ChatFrameMessage | null {
  if (event.origin !== expectedOrigin || !frameWindow || event.source !== frameWindow) return null
  const data = event.data as { type?: unknown; path?: unknown } | null
  if (!data || typeof data !== 'object') return null
  switch (data.type) {
    case 'chat:ready':
    case 'chat:balance-changed':
    case 'chat:session-expired':
      return { type: data.type }
    case 'chat:navigate':
      return isAllowedChatNavigatePath(data.path) ? { type: 'chat:navigate', path: data.path } : null
    default:
      return null
  }
}
