import { describe, expect, it } from 'vitest'
import { buildSession, isAllowedNavigatePath, parseNovaMessage } from '../studioBridge'

const ORIGIN = 'https://site.example'
const frameWin = {}

describe('studioBridge', () => {
  it('buildSession 需要令牌与用户，角色只有 admin / user', () => {
    expect(buildSession(1, null, { id: 1 }, false)).toBeNull()
    expect(buildSession(1, 'tok', null, false)).toBeNull()
    expect(buildSession(3, 'tok', { id: 7, role: 'admin' }, true)).toEqual({
      type: 'sub2api:nova-studio-session',
      revision: 3,
      sessionToken: 'tok',
      userId: 7,
      role: 'admin',
      theme: 'dark'
    })
    expect(buildSession(1, 'tok', { id: 7, role: 'superuser' }, false)?.role).toBe('user')
  })

  it('只接受同源且来自工作台 iframe 的消息', () => {
    const ok = { origin: ORIGIN, source: frameWin, data: { type: 'nova:ready' } } as unknown as MessageEvent
    expect(parseNovaMessage(ok, ORIGIN, frameWin)).toEqual({ type: 'nova:ready' })
    expect(parseNovaMessage({ ...ok, origin: 'https://evil.example' } as MessageEvent, ORIGIN, frameWin)).toBeNull()
    expect(parseNovaMessage({ ...ok, source: {} } as unknown as MessageEvent, ORIGIN, frameWin)).toBeNull()
    expect(parseNovaMessage(ok, ORIGIN, null)).toBeNull()
    expect(parseNovaMessage({ ...ok, data: 'nova:ready' } as MessageEvent, ORIGIN, frameWin)).toBeNull()
    expect(parseNovaMessage({ ...ok, data: { type: 'other' } } as MessageEvent, ORIGIN, frameWin)).toBeNull()
  })

  it('跳转只允许白名单内的站内路径', () => {
    const nav = (path: unknown) =>
      parseNovaMessage({ origin: ORIGIN, source: frameWin, data: { type: 'nova:navigate', path } } as unknown as MessageEvent, ORIGIN, frameWin)
    expect(nav('/purchase')).toEqual({ type: 'nova:navigate', path: '/purchase' })
    expect(nav('/admin/users')).toBeNull()
    expect(nav('https://evil.example')).toBeNull()
    expect(nav('//evil.example')).toBeNull()
    expect(isAllowedNavigatePath(undefined)).toBe(false)
  })
})
