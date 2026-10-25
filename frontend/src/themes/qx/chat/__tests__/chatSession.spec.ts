import { beforeEach, describe, expect, it, vi } from 'vitest'

const client = vi.hoisted(() => ({ post: vi.fn(), delete: vi.fn(), get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))

import {
  CHAT_OWNER_KEY,
  chatOwnedByOther,
  claimChatLocalData,
  issueChatSession,
  onChatRevoke,
  revokeChatSession
} from '../chatSession'

describe('chatSession', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    client.post.mockReset().mockResolvedValue({ data: {} })
    client.delete.mockReset().mockResolvedValue({ data: {} })
  })

  it('签发会话走 POST /chat/session', async () => {
    await issueChatSession()
    expect(client.post).toHaveBeenCalledWith('/chat/session')
  })

  it('revoke：先通知 iframe，再清 owui: 数据与归属标记，最后 DELETE Cookie', async () => {
    const order: string[] = []
    const handler = vi.fn(() => order.push('frame'))
    client.delete.mockImplementation(async () => {
      order.push('delete')
      expect(localStorage.getItem('owui:token')).toBeNull()
      return { data: {} }
    })
    const off = onChatRevoke(handler)
    localStorage.setItem('owui:token', 'x')
    localStorage.setItem('owui:theme', 'dark')
    localStorage.setItem('auth_token', 'keep')
    localStorage.setItem('theme', 'keep')
    localStorage.setItem(CHAT_OWNER_KEY, '7')
    sessionStorage.setItem('owui:draft', 'y')
    sessionStorage.setItem('auth_expired', '1')

    await revokeChatSession()

    expect(order).toEqual(['frame', 'delete'])
    expect(client.delete).toHaveBeenCalledWith('/chat/session', expect.objectContaining({ timeout: expect.any(Number) }))
    expect(localStorage.getItem('owui:token')).toBeNull()
    expect(localStorage.getItem('owui:theme')).toBeNull()
    expect(sessionStorage.getItem('owui:draft')).toBeNull()
    expect(localStorage.getItem(CHAT_OWNER_KEY)).toBeNull()
    expect(localStorage.getItem('auth_token')).toBe('keep')
    expect(localStorage.getItem('theme')).toBe('keep')
    expect(sessionStorage.getItem('auth_expired')).toBe('1')

    off()
    await revokeChatSession()
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('revoke 永不抛错：DELETE 失败或页面回调出错都不阻止退出', async () => {
    client.delete.mockRejectedValue({ status: 503 })
    const off = onChatRevoke(() => {
      throw new Error('boom')
    })
    await expect(revokeChatSession()).resolves.toBeUndefined()
    expect(client.delete).toHaveBeenCalledTimes(1)
    off()
  })

  it('本地数据归属：换了用户先清空，同一用户保留', () => {
    localStorage.setItem('owui:chat', 'a')
    claimChatLocalData(1)
    expect(localStorage.getItem('owui:chat')).toBeNull()
    expect(localStorage.getItem(CHAT_OWNER_KEY)).toBe('1')

    localStorage.setItem('owui:chat', 'a2')
    claimChatLocalData(1)
    expect(localStorage.getItem('owui:chat')).toBe('a2')

    claimChatLocalData(2)
    expect(localStorage.getItem('owui:chat')).toBeNull()
    expect(localStorage.getItem(CHAT_OWNER_KEY)).toBe('2')
  })

  it('归属判断：没有记录不算账号变化，记录为别的用户才算', () => {
    expect(chatOwnedByOther(1)).toBe(false)
    localStorage.setItem(CHAT_OWNER_KEY, '1')
    expect(chatOwnedByOther(1)).toBe(false)
    expect(chatOwnedByOther(2)).toBe(true)
  })
})
