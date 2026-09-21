import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, post } }))

import { bindInviter, lookupUsers, type BindInviterRequest, type BindInviterResponse } from '@/api/admin/affiliates'

describe('admin affiliates api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('posts a one-time inviter binding request and returns binding metadata', async () => {
    const payload: BindInviterRequest = { inviter_id: 22, reason: 'registration omission' }
    const response: BindInviterResponse = {
      invitee_id: 11,
      inviter_id: 22,
      bound_at: '2026-09-21T12:00:00Z',
      source: 'admin',
    }
    post.mockResolvedValue({ data: response })

    await expect(bindInviter(11, payload)).resolves.toEqual(response)
    expect(post).toHaveBeenCalledWith('/admin/affiliates/users/11/inviter', payload)
  })

  it('forwards cancellation to user lookup requests', async () => {
    const controller = new AbortController()
    get.mockResolvedValue({ data: [] })

    await expect(lookupUsers('user', { signal: controller.signal })).resolves.toEqual([])
    expect(get).toHaveBeenCalledWith('/admin/affiliates/users/lookup', {
      params: { q: 'user' },
      signal: controller.signal,
    })
  })
})
