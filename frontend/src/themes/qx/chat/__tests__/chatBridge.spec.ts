import { describe, expect, it } from 'vitest'
import {
  buildChatSession,
  chatFrameSrc,
  isAllowedChatNavigatePath,
  parseChatMessage,
  toChatLocale
} from '../chatBridge'

const ORIGIN = 'https://site.example'
const frameWin = {}
const ev = (data: unknown, extra: Partial<{ origin: string; source: unknown }> = {}) =>
  ({ origin: ORIGIN, source: frameWin, data, ...extra }) as unknown as MessageEvent

describe('chatBridge', () => {
  it('chat-session 只带 revision / 主题 / 语言，不带任何令牌', () => {
    expect(buildChatSession(3, true, 'zh-CN')).toEqual({
      type: 'sub2api:chat-session',
      revision: 3,
      theme: 'dark',
      locale: 'zh-CN'
    })
    expect(buildChatSession(1, false, 'en-US').theme).toBe('light')
  })

  it('控制台语言映射为 zh-CN / en-US，iframe 地址带 lang 与 theme', () => {
    expect(toChatLocale('zh')).toBe('zh-CN')
    expect(toChatLocale('zh-TW')).toBe('zh-CN')
    expect(toChatLocale('en')).toBe('en-US')
    expect(toChatLocale(undefined)).toBe('en-US')
    expect(chatFrameSrc('zh-CN', 'light')).toBe('/_chat/?lang=zh-CN&theme=light')
    expect(chatFrameSrc('en-US', 'dark')).toBe('/_chat/?lang=en-US&theme=dark')
  })

  it('只接受同源且来自对话 iframe 的消息', () => {
    expect(parseChatMessage(ev({ type: 'chat:ready' }), ORIGIN, frameWin)).toEqual({ type: 'chat:ready' })
    expect(parseChatMessage(ev({ type: 'chat:balance-changed' }), ORIGIN, frameWin)).toEqual({
      type: 'chat:balance-changed'
    })
    expect(parseChatMessage(ev({ type: 'chat:session-expired' }), ORIGIN, frameWin)).toEqual({
      type: 'chat:session-expired'
    })
    expect(parseChatMessage(ev({ type: 'chat:session-expired' }, { source: {} }), ORIGIN, frameWin)).toBeNull()
    expect(parseChatMessage(ev({ type: 'chat:ready' }, { origin: 'https://evil.example' }), ORIGIN, frameWin)).toBeNull()
    expect(parseChatMessage(ev({ type: 'chat:ready' }, { origin: 'null' }), ORIGIN, frameWin)).toBeNull()
    expect(parseChatMessage(ev({ type: 'chat:ready' }, { source: {} }), ORIGIN, frameWin)).toBeNull()
    expect(parseChatMessage(ev({ type: 'chat:ready' }, { source: null }), ORIGIN, frameWin)).toBeNull()
    expect(parseChatMessage(ev({ type: 'chat:ready' }), ORIGIN, null)).toBeNull()
    expect(parseChatMessage(ev({ type: 'chat:ready' }), ORIGIN, undefined)).toBeNull()
    expect(parseChatMessage(ev('chat:ready'), ORIGIN, frameWin)).toBeNull()
    expect(parseChatMessage(ev(null), ORIGIN, frameWin)).toBeNull()
    expect(parseChatMessage(ev({ type: 'sub2api:chat-session' }), ORIGIN, frameWin)).toBeNull()
    expect(parseChatMessage(ev({ type: 'nova:ready' }), ORIGIN, frameWin)).toBeNull()
  })

  it('跳转只允许 /purchase 与 /keys', () => {
    const nav = (path: unknown) => parseChatMessage(ev({ type: 'chat:navigate', path }), ORIGIN, frameWin)
    expect(nav('/purchase')).toEqual({ type: 'chat:navigate', path: '/purchase' })
    expect(nav('/keys')).toEqual({ type: 'chat:navigate', path: '/keys' })
    for (const bad of ['/usage', '/admin/users', '/purchase?x=1', '/keys/', 'https://evil.example', '//evil.example', 'javascript:alert(1)', '', 1, null, undefined]) {
      expect(nav(bad)).toBeNull()
    }
    expect(isAllowedChatNavigatePath(undefined)).toBe(false)
  })
})
