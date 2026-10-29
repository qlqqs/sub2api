import { beforeEach, describe, expect, it, vi } from 'vitest'
const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post } }))
import { getAffiliateSignupBonus } from '@/api/auth'

describe('signup gift preview', () => {
  beforeEach(() => { post.mockReset() })
  it('sends the referral code and cancellation signal', async () => {
    post.mockResolvedValue({ data: { amount: 10 } })
    const controller = new AbortController()
    await expect(getAffiliateSignupBonus('CODE1', controller.signal)).resolves.toBe(10)
    expect(post).toHaveBeenCalledWith('/auth/affiliate-signup-bonus', { aff_code: 'CODE1' }, { signal: controller.signal })
  })
  it.each([0, -5, undefined, 'invalid', Infinity])('normalizes invalid or missing amounts: %s', async amount => {
    post.mockResolvedValue({ data: { amount } })
    await expect(getAffiliateSignupBonus('CODE1')).resolves.toBe(0)
  })
  it('leaves failures to the caller rather than using the global amount', async () => {
    post.mockRejectedValue(new Error('offline'))
    await expect(getAffiliateSignupBonus('CODE1')).rejects.toThrow('offline')
  })
})
