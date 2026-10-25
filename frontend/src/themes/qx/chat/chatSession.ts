/**
 * CUSTOM: 对话工作台的会话 Cookie（s2a_chat，HttpOnly，Path=/_chat）与退出 / 换账号时的撤销。
 *
 * - POST /api/v1/chat/session：用当前 access token 签发 Cookie；进入 /chat 前与 token 刷新后调用。
 * - DELETE /api/v1/chat/session：清 s2a_chat 与 Open WebUI 自己的 token Cookie，不要求登录有效；
 *   退出时调用，发现账号变化时先 DELETE 再 POST。
 * - revokeChatSession()：由 auth store 的 logout 调用。先通知打开着的 iframe（sub2api:chat-revoke），
 *   再清掉 iframe 写在同源存储里的 owui: 前缀数据，最后清 Cookie；任何一步失败都不阻止退出。
 */
import { apiClient } from '@/api/client'

/** 同源存储里 Open WebUI 的键前缀（fork 统一加的，见 design §6）。 */
export const CHAT_STORAGE_PREFIX = 'owui:'
/** 记录本浏览器里对话本地数据属于哪个 sub2api 用户；换人时先清空再加载 iframe。 */
export const CHAT_OWNER_KEY = 's2a_chat_owner'

const DELETE_TIMEOUT_MS = 5000

export async function issueChatSession(): Promise<void> {
  await apiClient.post('/chat/session')
}

export async function deleteChatSession(): Promise<void> {
  await apiClient.delete('/chat/session', { timeout: DELETE_TIMEOUT_MS })
}

function clearPrefixed(storage: Storage | undefined): void {
  if (!storage) return
  const keys: string[] = []
  for (let i = 0; i < storage.length; i++) {
    const key = storage.key(i)
    if (key?.startsWith(CHAT_STORAGE_PREFIX)) keys.push(key)
  }
  keys.forEach((key) => storage.removeItem(key))
}

/** 清掉 iframe 留在同源 localStorage / sessionStorage 里的数据（草稿、设置、缓存）。 */
export function clearChatLocalData(): void {
  try {
    clearPrefixed(typeof localStorage === 'undefined' ? undefined : localStorage)
  } catch {
    // 存储不可用（隐私模式等）时忽略
  }
  try {
    clearPrefixed(typeof sessionStorage === 'undefined' ? undefined : sessionStorage)
  } catch {
    // 同上
  }
}

/**
 * 本地对话数据（及 Cookie）是否记在另一个用户名下。
 * 没有记录（首次使用，或正常退出时已清掉）不算账号变化。
 */
export function chatOwnedByOther(userId: number): boolean {
  try {
    const owner = localStorage.getItem(CHAT_OWNER_KEY)
    return owner !== null && owner !== String(userId)
  } catch {
    return false
  }
}

/**
 * 本地数据归属：与当前用户不同（含首次使用）时先清空，再记为当前用户。
 * 用于没走正常退出（如令牌失效被跳回登录页）就换了账号的情况。
 */
export function claimChatLocalData(userId: number): void {
  try {
    if (localStorage.getItem(CHAT_OWNER_KEY) === String(userId)) return
    clearChatLocalData()
    localStorage.setItem(CHAT_OWNER_KEY, String(userId))
  } catch {
    clearChatLocalData()
  }
}

type RevokeHandler = () => void
const revokeHandlers = new Set<RevokeHandler>()

/** ChatView 挂载时注册：退出登录时同步撤销 iframe。返回取消注册的函数。 */
export function onChatRevoke(handler: RevokeHandler): () => void {
  revokeHandlers.add(handler)
  return () => {
    revokeHandlers.delete(handler)
  }
}

/** 退出登录 / 切换账号：撤销 iframe、清本地数据、清 Cookie。永不抛错。 */
export async function revokeChatSession(): Promise<void> {
  for (const handler of [...revokeHandlers]) {
    try {
      handler()
    } catch {
      // 单个页面出错不影响退出
    }
  }
  clearChatLocalData()
  try {
    localStorage.removeItem(CHAT_OWNER_KEY)
  } catch {
    // ignore
  }
  try {
    await deleteChatSession()
  } catch {
    // 未启用（503）、网络失败都不阻止退出
  }
}
