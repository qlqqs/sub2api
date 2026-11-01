import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))

import {
  getOverview,
  getStatement,
  listStatements,
  listTeamLines,
  normalizeAffiliatePartnerInfo,
  normalizePartnerMonthTotals,
  normalizePartnerStatementDetail,
  partnerAgentTeamAPI,
  partnerRegionTeamAPI
} from '@/api/affiliatePartner'

const RANGE = { start_date: '2026-11-01', end_date: '2026-11-07' }

describe('affiliate partner api (CUSTOM)', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('keeps amount strings and fills missing month fields', () => {
    const totals = normalizePartnerMonthTotals({
      month: '2026-11',
      status: 'draft',
      share_rate: '0.4000',
      revenue: '96.80000000',
      profit: '-3.2',
      share_amount: '0.00',
      paid_on: ''
    })
    expect(totals).toMatchObject({
      month: '2026-11',
      status: 'draft',
      share_rate: '0.4000',
      revenue: '96.80000000',
      profit: '-3.2',
      account_cost: '0',
      loss_before: '0',
      share_amount: '0.00',
      confirmed_at: null,
      paid_on: null,
      paid_note: null
    })
    expect(normalizePartnerMonthTotals({ status: 'weird' }).status).toBe('current')
  })

  it('normalizes a statement: lines, cost splits, adjustments and warnings', () => {
    const detail = normalizePartnerStatementDetail({
      totals: { month: '2026-10', status: 'confirmed' },
      lines: [
        { line_agent: null, members: 3, revenue: '10' },
        { line_agent: { id: 59, email: 'a***@x.com', username: 'agent', deleted: false }, members: '6', profit: '1.5' }
      ],
      cost_by_group: [{ group_id: 2, group_name: 'Codex', consumption: '5', account_cost: '1' }],
      cost_by_model: [{ model: 'gpt-6-sol', account_cost: '1' }],
      adjustments: [{ id: 1, amount: '-5.00', note: '补差', created_at: '2026-11-01T00:00:00+08:00' }],
      warnings: ['2 条请求走固定成本账号，成本按 0 计', 3]
    })
    expect(detail.totals.status).toBe('confirmed')
    expect(detail.lines[0]).toMatchObject({ line_agent: null, members: 3, revenue: '10', profit: '0' })
    expect(detail.lines[1].line_agent).toEqual({ id: 59, email: 'a***@x.com', username: 'agent', deleted: false })
    expect(detail.lines[1].members).toBe(6)
    expect(detail.cost_by_group[0]).toEqual({ group_id: 2, group_name: 'Codex', consumption: '5', account_cost: '1' })
    expect(detail.cost_by_model[0]).toEqual({ model: 'gpt-6-sol', consumption: '0', account_cost: '1' })
    expect(detail.adjustments[0]).toMatchObject({ id: 1, amount: '-5.00', note: '补差', created_by_email: null })
    expect(detail.warnings).toEqual(['2 条请求走固定成本账号，成本按 0 计'])
  })

  it('reads the partner field of /user/aff', () => {
    expect(normalizeAffiliatePartnerInfo(null)).toBeNull()
    expect(normalizeAffiliatePartnerInfo(undefined)).toBeNull()
    expect(
      normalizeAffiliatePartnerInfo({ region_name: '三科大', share_rate: '0.4000', next_share_rate: '0.3500', next_rate_month: '2026-12', since: 's' })
    ).toEqual({ region_name: '三科大', share_rate: '0.4000', next_share_rate: '0.3500', next_rate_month: '2026-12', since: 's' })
  })

  it('requests the partner overview and statements', async () => {
    get.mockResolvedValueOnce({ data: { region_name: '重大', agent_count: 2, current_month: { month: '2026-11' }, lines: [{}] } })
    const overview = await getOverview()
    expect(get).toHaveBeenLastCalledWith('/user/aff/partner/overview', { signal: undefined })
    expect(overview).toMatchObject({ region_name: '重大', agent_count: 2, loss_carried: '0', next_share_rate: null })
    expect(overview.current_month.month).toBe('2026-11')
    expect(overview.lines[0].line_agent).toBeNull()

    get.mockResolvedValueOnce({ data: { items: [{ month: '2026-11', status: 'current' }] } })
    expect((await listStatements())[0].status).toBe('current')
    expect(get).toHaveBeenLastCalledWith('/user/aff/partner/statements', { signal: undefined })

    get.mockResolvedValueOnce({ data: {} })
    await getStatement('2026-10')
    expect(get).toHaveBeenLastCalledWith('/user/aff/partner/statements/2026-10', { signal: undefined })
  })

  it('maps region customers and lines to the team page user shape', async () => {
    get.mockResolvedValueOnce({
      data: {
        items: [
          {
            user: { user_id: 9, nickname: 'c', email: 'c***@x.com', deleted: false },
            cost: 1.5,
            owner: { id: 80, email: 'd***@x.com', username: 'dist', deleted: false },
            line_agent: null
          }
        ],
        total: 1,
        page: 1,
        page_size: 20,
        pages: 1
      }
    })
    const customers = await partnerRegionTeamAPI.listCustomers({ ...RANGE, page: 1 })
    expect(get).toHaveBeenLastCalledWith('/user/aff/partner/team/customers', { params: { ...RANGE, page: 1 }, signal: undefined })
    expect(customers.items[0].owner).toEqual({ user_id: 80, nickname: 'dist', email: 'd***@x.com', deleted: false })
    expect(customers.items[0].line_agent).toBeNull()
    expect(customers.items[0].cost).toBe(1.5)

    get.mockResolvedValueOnce({
      data: {
        items: [
          { line_agent: null, customers: 2, active_customers: 1, requests: 5, team_cost: 3.2, cost_share: 0.4 },
          { line_agent: { id: 59, email: 'a***@x.com', username: 'agent', deleted: false }, customers: 4 }
        ]
      }
    })
    const lines = await listTeamLines(RANGE)
    expect(get).toHaveBeenLastCalledWith('/user/aff/partner/team/lines', { params: RANGE, signal: undefined })
    expect(lines[0]).toEqual({ line_agent: null, customers: 2, active_customers: 1, requests: 5, team_cost: 3.2, cost_share: 0.4 })
    expect(lines[1]).toMatchObject({ line_agent: { user_id: 59, nickname: 'agent' }, customers: 4, team_cost: 0 })
  })

  it('routes the region overview / requests and the attached agent view', async () => {
    get.mockResolvedValue({ data: { items: [] } })
    await partnerRegionTeamAPI.getOverview({ ...RANGE, user_id: 9 })
    expect(get).toHaveBeenLastCalledWith('/user/aff/partner/team/overview', { params: { ...RANGE, user_id: 9 }, signal: undefined })
    await partnerRegionTeamAPI.listRequests(9, RANGE)
    expect(get).toHaveBeenLastCalledWith('/user/aff/partner/team/customers/9/requests', { params: RANGE, signal: undefined })

    let agentId = 59
    const agentAPI = partnerAgentTeamAPI(() => agentId)
    await agentAPI.getOverview(RANGE)
    expect(get).toHaveBeenLastCalledWith('/user/aff/partner/team/agents/59/overview', { params: RANGE, signal: undefined })
    agentId = 60
    await agentAPI.listAgents(RANGE)
    expect(get).toHaveBeenLastCalledWith('/user/aff/partner/team/agents/60/agents', { params: RANGE, signal: undefined })
    await agentAPI.listCustomers(RANGE)
    expect(get).toHaveBeenLastCalledWith('/user/aff/partner/team/agents/60/customers', { params: RANGE, signal: undefined })
    await agentAPI.listRequests(7, RANGE)
    expect(get).toHaveBeenLastCalledWith('/user/aff/partner/team/agents/60/customers/7/requests', { params: RANGE, signal: undefined })
  })
})
