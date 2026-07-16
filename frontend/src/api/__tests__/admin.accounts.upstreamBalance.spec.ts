import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { post }
}))

import { queryUpstreamBalance } from '@/api/admin/accounts'

describe('admin upstream account balance API', () => {
  beforeEach(() => {
    post.mockReset()
  })

  it('posts to the single-account balance action and returns unwrapped data', async () => {
    const result = {
      status: 'available',
      platform_type: 'new_api',
      scope: 'user',
      remaining: '10',
      unit: 'USD',
      queried_at: '2026-07-16T12:00:00Z'
    }
    post.mockResolvedValue({ data: result })

    await expect(queryUpstreamBalance(42)).resolves.toEqual(result)
    expect(post).toHaveBeenCalledWith('/admin/accounts/42/upstream-balance')
  })
})
