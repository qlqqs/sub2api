import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h, reactive, ref } from 'vue'

const state = vi.hoisted(() => ({
  client: { post: vi.fn(), delete: vi.fn(), get: vi.fn() },
  push: vi.fn(),
  enabled: null as unknown as import('vue').Ref<boolean | undefined>,
  auth: null as unknown as {
    token: string | null
    user: { id: number; role: string } | null
    refreshUser: ReturnType<typeof vi.fn>
  },
  locale: null as unknown as import('vue').Ref<string>
}))

vi.mock('@/api/client', () => ({ apiClient: state.client }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => state.auth }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: state.push }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: state.locale }) }))
vi.mock('@/components/layout', () => ({
  AppLayout: defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) })
}))
vi.mock('../chatStatus', () => ({ useChatEnabled: () => state.enabled }))

import ChatView from '../ChatView.vue'
import { revokeChatSession } from '../chatSession'

const ORIGIN = window.location.origin
let wrapper: VueWrapper | null = null

function setStoredAuth(token: string | null, userId: number | null) {
  if (token) localStorage.setItem('auth_token', token)
  else localStorage.removeItem('auth_token')
  if (userId !== null) localStorage.setItem('auth_user', JSON.stringify({ id: userId }))
  else localStorage.removeItem('auth_user')
}

async function mountView() {
  wrapper = mount(ChatView, { attachTo: document.body })
  await flushPromises()
  return wrapper
}

function frameWindow(): Window {
  const el = wrapper!.find('iframe').element as HTMLIFrameElement
  return el.contentWindow!
}

function fromFrame(data: unknown, source: unknown = frameWindow(), origin = ORIGIN) {
  window.dispatchEvent(new MessageEvent('message', { data, origin, source: source as Window }))
}

describe('ChatView', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    state.client.post.mockReset().mockResolvedValue({ data: { expires_at: 1 } })
    state.client.delete.mockReset().mockResolvedValue({ data: {} })
    state.push.mockReset()
    state.enabled = ref<boolean | undefined>(true)
    state.locale = ref('zh')
    state.auth = reactive({ token: 'tok-1', user: { id: 7, role: 'user' }, refreshUser: vi.fn().mockResolvedValue({}) })
    setStoredAuth('tok-1', 7)
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
  })

  it('先签发会话 Cookie 再加载 iframe，地址带语言', async () => {
    let resolve!: () => void
    state.client.post.mockReturnValue(new Promise<void>((r) => (resolve = r)))
    wrapper = mount(ChatView, { attachTo: document.body })
    await flushPromises()
    expect(state.client.post).toHaveBeenCalledWith('/chat/session')
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.find('[data-test="chat-loading"]').exists()).toBe(true)

    resolve()
    await flushPromises()
    const iframe = wrapper.find('iframe')
    expect(iframe.attributes('src')).toBe('/_chat/?lang=zh-CN&theme=light')
    expect(iframe.attributes('title')).toBe('qxTheme.chat.frameTitle')
    expect(state.client.delete).not.toHaveBeenCalled() // 普通首次加载不 DELETE
  })

  it('iframe 地址带当前深浅色（首屏预置）', async () => {
    document.documentElement.classList.add('dark')
    state.locale.value = 'en'
    await mountView()
    expect(wrapper!.find('iframe').attributes('src')).toBe('/_chat/?lang=en-US&theme=dark')
    document.documentElement.classList.remove('dark')
  })

  it('签发失败显示错误、不加载 iframe，重试成功后加载', async () => {
    state.client.post.mockRejectedValueOnce({ status: 503 })
    await mountView()
    expect(wrapper!.find('[data-test="chat-error"]').exists()).toBe(true)
    expect(wrapper!.find('iframe').exists()).toBe(false)

    await wrapper!.find('[data-test="chat-retry"]').trigger('click')
    await flushPromises()
    expect(wrapper!.find('iframe').exists()).toBe(true)
  })

  it('未开启时显示「未开启」，不签发会话', async () => {
    state.enabled.value = false
    await mountView()
    expect(wrapper!.find('[data-test="chat-disabled"]').text()).toBe('qxTheme.chat.disabled')
    expect(state.client.post).not.toHaveBeenCalled()
    expect(wrapper!.find('iframe').exists()).toBe(false)
  })

  it('状态加载完成后才签发', async () => {
    state.enabled.value = undefined
    await mountView()
    expect(state.client.post).not.toHaveBeenCalled()
    state.enabled.value = true
    await flushPromises()
    expect(state.client.post).toHaveBeenCalledTimes(1)
    expect(wrapper!.find('iframe').exists()).toBe(true)
  })

  it('chat:ready → 回 chat-session（同源 targetOrigin）；伪造来源忽略', async () => {
    await mountView()
    const win = frameWindow()
    const spy = vi.spyOn(win, 'postMessage')

    fromFrame({ type: 'chat:ready' }, window)
    fromFrame({ type: 'chat:ready' }, win, 'https://evil.example')
    expect(spy).not.toHaveBeenCalled()

    fromFrame({ type: 'chat:ready' })
    expect(spy).toHaveBeenCalledWith(
      { type: 'sub2api:chat-session', revision: 1, theme: 'light', locale: 'zh-CN' },
      ORIGIN
    )
  })

  it('主题 / 语言变化时重发；语言变化不重载 iframe', async () => {
    await mountView()
    const spy = vi.spyOn(frameWindow(), 'postMessage')
    document.documentElement.classList.add('dark')
    await flushPromises()
    expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ theme: 'dark', locale: 'zh-CN' }), ORIGIN)

    state.locale.value = 'en'
    await flushPromises()
    expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ theme: 'dark', locale: 'en-US' }), ORIGIN)
    expect(wrapper!.find('iframe').attributes('src')).toBe('/_chat/?lang=zh-CN&theme=light')
    document.documentElement.classList.remove('dark')
  })

  it('token 刷新后重新签发 Cookie，revision + 1', async () => {
    await mountView()
    const spy = vi.spyOn(frameWindow(), 'postMessage')
    setStoredAuth('tok-2', 7)
    state.auth.token = 'tok-2'
    await flushPromises()
    expect(state.client.post).toHaveBeenCalledTimes(2)
    expect(state.client.delete).not.toHaveBeenCalled() // token 刷新不 DELETE
    expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'sub2api:chat-session', revision: 2 }), ORIGIN)
  })

  it('chat:session-expired → 重新签发（不 DELETE），revision + 1；并发只签一次；伪造来源忽略', async () => {
    await mountView()
    const spy = vi.spyOn(frameWindow(), 'postMessage')
    fromFrame({ type: 'chat:session-expired' }, window)
    await flushPromises()
    expect(state.client.post).toHaveBeenCalledTimes(1)

    fromFrame({ type: 'chat:session-expired' })
    fromFrame({ type: 'chat:session-expired' })
    await flushPromises()
    expect(state.client.post).toHaveBeenCalledTimes(2)
    expect(state.client.delete).not.toHaveBeenCalled()
    expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'sub2api:chat-session', revision: 2 }), ORIGIN)

    fromFrame({ type: 'chat:session-expired' })
    await flushPromises()
    expect(state.client.post).toHaveBeenCalledTimes(3)
  })

  it('chat:balance-changed 刷新顶栏余额', async () => {
    await mountView()
    fromFrame({ type: 'chat:balance-changed' })
    expect(state.auth.refreshUser).toHaveBeenCalledTimes(1)
  })

  it('chat:navigate 只跳白名单', async () => {
    await mountView()
    fromFrame({ type: 'chat:navigate', path: '/admin/users' })
    fromFrame({ type: 'chat:navigate', path: 'https://evil.example' })
    expect(state.push).not.toHaveBeenCalled()
    fromFrame({ type: 'chat:navigate', path: '/purchase' })
    fromFrame({ type: 'chat:navigate', path: '/keys' })
    expect(state.push.mock.calls).toEqual([['/purchase'], ['/keys']])
  })

  it('退出登录：先发 chat-revoke，卸载 iframe，清 owui: 数据并 DELETE Cookie', async () => {
    await mountView()
    const spy = vi.spyOn(frameWindow(), 'postMessage')
    localStorage.setItem('owui:token', 'x')

    await revokeChatSession()
    expect(spy).toHaveBeenCalledWith({ type: 'sub2api:chat-revoke' }, ORIGIN)
    expect(state.client.delete).toHaveBeenCalledWith('/chat/session', expect.anything())
    expect(localStorage.getItem('owui:token')).toBeNull()
    await flushPromises()
    expect(wrapper!.find('iframe').exists()).toBe(false)
  })

  it('同页换账号：撤销后为新用户重新签发并重载 iframe', async () => {
    await mountView()
    const spy = vi.spyOn(frameWindow(), 'postMessage')
    localStorage.setItem('owui:chat', 'old-user')

    setStoredAuth('tok-b', 8)
    state.auth.token = 'tok-b'
    state.auth.user = { id: 8, role: 'user' }
    await flushPromises()

    expect(spy).toHaveBeenCalledWith({ type: 'sub2api:chat-revoke' }, ORIGIN)
    expect(localStorage.getItem('owui:chat')).toBeNull()
    expect(state.client.delete).toHaveBeenCalledTimes(1)
    expect(state.client.post).toHaveBeenCalledTimes(2)
    // 先 DELETE（清旧用户的 s2a_chat 与 Open WebUI token Cookie）再 POST
    expect(state.client.delete.mock.invocationCallOrder[0]).toBeLessThan(state.client.post.mock.invocationCallOrder[1])
    expect(state.client.delete.mock.invocationCallOrder[0]).toBeGreaterThan(state.client.post.mock.invocationCallOrder[0])
    expect(wrapper!.find('iframe').exists()).toBe(true)
    expect(localStorage.getItem('s2a_chat_owner')).toBe('8')
  })

  it('加载前发现本地对话数据属于别的用户：先 DELETE 再 POST，清空本地数据后加载', async () => {
    localStorage.setItem('s2a_chat_owner', '3')
    localStorage.setItem('owui:chat', 'other')
    await mountView()
    expect(state.client.delete).toHaveBeenCalledWith('/chat/session', expect.anything())
    expect(state.client.delete.mock.invocationCallOrder[0]).toBeLessThan(state.client.post.mock.invocationCallOrder[0])
    expect(localStorage.getItem('owui:chat')).toBeNull()
    expect(localStorage.getItem('s2a_chat_owner')).toBe('7')
    expect(wrapper!.find('iframe').exists()).toBe(true)
  })

  it('账号变化时 DELETE 失败：显示错误、不 POST、不加载；重试仍先 DELETE', async () => {
    localStorage.setItem('s2a_chat_owner', '3')
    localStorage.setItem('owui:chat', 'other')
    state.client.delete.mockRejectedValueOnce({ status: 0 })
    await mountView()
    expect(wrapper!.find('[data-test="chat-error"]').exists()).toBe(true)
    expect(state.client.post).not.toHaveBeenCalled()
    expect(wrapper!.find('iframe').exists()).toBe(false)
    expect(localStorage.getItem('s2a_chat_owner')).toBe('3')

    await wrapper!.find('[data-test="chat-retry"]').trigger('click')
    await flushPromises()
    expect(state.client.delete).toHaveBeenCalledTimes(2)
    expect(state.client.post).toHaveBeenCalledTimes(1)
    expect(wrapper!.find('iframe').exists()).toBe(true)
  })

  it('同一用户再次进入：不 DELETE', async () => {
    localStorage.setItem('s2a_chat_owner', '7')
    localStorage.setItem('owui:chat', 'mine')
    await mountView()
    expect(state.client.delete).not.toHaveBeenCalled()
    expect(localStorage.getItem('owui:chat')).toBe('mine')
  })

  it('其他标签页换了账号：撤销并提示刷新', async () => {
    await mountView()
    const spy = vi.spyOn(frameWindow(), 'postMessage')
    setStoredAuth('tok-b', 8)
    window.dispatchEvent(new StorageEvent('storage', { key: 'auth_user', newValue: JSON.stringify({ id: 8 }) }))
    await flushPromises()
    expect(spy).toHaveBeenCalledWith({ type: 'sub2api:chat-revoke' }, ORIGIN)
    expect(wrapper!.find('iframe').exists()).toBe(false)
    expect(wrapper!.find('[data-test="chat-stale"]').exists()).toBe(true)
  })

  it('陈旧的签发响应不会在换账号后加载 iframe', async () => {
    let resolveFirst!: () => void
    state.client.post.mockReturnValueOnce(new Promise<void>((r) => (resolveFirst = r)))
    wrapper = mount(ChatView, { attachTo: document.body })
    await flushPromises()
    await revokeChatSession()
    resolveFirst()
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(false)
  })
})
