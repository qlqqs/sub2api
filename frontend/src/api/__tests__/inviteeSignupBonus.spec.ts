import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getPublicSettings } from '@/api/auth'
import { getAffiliateDetail } from '@/api/user'
import { getSettings, updateSettings } from '@/api/admin/settings'
import { AFFILIATE_REWARD_TYPES, getUserOverview, listRebateRecords } from '@/api/admin/affiliates'

const { get, put } = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, put } }))

beforeEach(() => vi.clearAllMocks())

describe('注册赠送接口契约', () => {
  it.each([undefined, null, -1, NaN, Infinity, '5'])('设置缺省或非法金额 %s 归零', async value => {
    get.mockResolvedValue({ data: { affiliate_enabled: true, affiliate_invitee_signup_bonus: value } })
    expect((await getPublicSettings()).affiliate_invitee_signup_bonus).toBe(0)
    expect((await getSettings()).affiliate_invitee_signup_bonus).toBe(0)
  })

  it('公开设置总开关关闭时隐藏金额，管理员仍能读取配置', async () => {
    get.mockResolvedValue({ data: { affiliate_enabled: false, affiliate_invitee_signup_bonus: 5 } })
    expect((await getPublicSettings()).affiliate_invitee_signup_bonus).toBe(0)
    expect((await getSettings()).affiliate_invitee_signup_bonus).toBe(5)
  })

  it('管理员提交独立金额，公开设置保留有效金额精度', async () => {
    const data = { affiliate_enabled: true, affiliate_invitee_signup_bonus: 5.12345678 }
    get.mockResolvedValue({ data })
    put.mockResolvedValue({ data })
    expect((await getPublicSettings()).affiliate_invitee_signup_bonus).toBe(5.12345678)
    expect((await updateSettings({ affiliate_invitee_signup_bonus: 5.12345678 })).affiliate_invitee_signup_bonus).toBe(5.12345678)
    expect(put).toHaveBeenCalledWith('/admin/settings', { affiliate_invitee_signup_bonus: 5.12345678 })
  })

  it('详情与汇总兼容旧接口缺省值，首充合计不受影响', async () => {
    get.mockResolvedValue({ data: { reward_totals: { agent: 1, invite: 2, invitee_bonus: 3 } } })
    expect(await getAffiliateDetail()).toMatchObject({ invitee_signup_bonus: 0, reward_totals: { agent: 1, invite: 2, invitee_bonus: 3, invitee_signup_bonus: 0 } })
    expect((await getUserOverview(10)).reward_totals).toEqual({ agent: 1, invite: 2, invitee_bonus: 3, invitee_signup_bonus: 0 })
  })

  it('详情保留独立赠送配置与已领取金额，缺省汇总不制造奖励', async () => {
    get.mockResolvedValueOnce({ data: { invitee_signup_bonus: 5, reward_totals: { agent: 1, invite: 2, invitee_bonus: 3, invitee_signup_bonus: 4 } } })
    expect(await getAffiliateDetail()).toMatchObject({ invitee_signup_bonus: 5, reward_totals: { invitee_bonus: 3, invitee_signup_bonus: 4 } })
    get.mockResolvedValue({ data: {} })
    expect((await getAffiliateDetail()).reward_totals).toBeNull()
    expect((await getUserOverview(10)).reward_totals).toBeNull()
  })

  it('后台返利筛选白名单和请求支持注册赠送', async () => {
    expect(AFFILIATE_REWARD_TYPES).toContain('invitee_signup_bonus')
    get.mockResolvedValue({ data: { items: [] } })
    await listRebateRecords({ reward_type: 'invitee_signup_bonus' })
    expect(get).toHaveBeenCalledWith('/admin/affiliates/rebates', expect.objectContaining({ params: expect.objectContaining({ reward_type: 'invitee_signup_bonus' }) }))
  })
})
