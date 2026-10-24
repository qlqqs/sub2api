/**
 * CUSTOM: 创作工作台（Nova Studio）父页与 iframe 之间的 postMessage 契约。
 * 完整契约见 qlqqs/nova-image-studio 的 docs/sub2api-embed.md §9。
 */

export const STUDIO_FRAME_SRC = '/_nova/?embedded=1'

export interface StudioSession {
  type: 'sub2api:nova-studio-session'
  revision: number
  sessionToken: string
  userId: number
  role: 'admin' | 'user'
  theme: 'light' | 'dark'
}

export interface StudioRevoke {
  type: 'sub2api:nova-studio-revoke'
}

export type NovaMessage =
  | { type: 'nova:ready' }
  | { type: 'nova:balance-changed' }
  | { type: 'nova:navigate'; path: string }

/** Nova 只能请求跳到这些站内页面（如余额不足时「去充值」）。 */
const NAVIGATE_ALLOWLIST = ['/purchase', '/keys', '/usage', '/orders', '/redeem', '/profile']

export function isAllowedNavigatePath(path: unknown): path is string {
  return typeof path === 'string' && NAVIGATE_ALLOWLIST.includes(path)
}

export function buildSession(
  revision: number,
  token: string | null | undefined,
  user: { id: number; role?: string } | null | undefined,
  dark: boolean
): StudioSession | null {
  if (!token || !user) return null
  return {
    type: 'sub2api:nova-studio-session',
    revision,
    sessionToken: token,
    userId: user.id,
    role: user.role === 'admin' ? 'admin' : 'user',
    theme: dark ? 'dark' : 'light'
  }
}

/** 只接受同源、且来自工作台 iframe 的消息；其余一律忽略。 */
export function parseNovaMessage(
  event: Pick<MessageEvent, 'origin' | 'source' | 'data'>,
  expectedOrigin: string,
  frameWindow: unknown
): NovaMessage | null {
  if (event.origin !== expectedOrigin || !frameWindow || event.source !== frameWindow) return null
  const data = event.data as { type?: unknown; path?: unknown } | null
  if (!data || typeof data !== 'object') return null
  switch (data.type) {
    case 'nova:ready':
    case 'nova:balance-changed':
      return { type: data.type }
    case 'nova:navigate':
      return isAllowedNavigatePath(data.path) ? { type: 'nova:navigate', path: data.path } : null
    default:
      return null
  }
}
