import { describe, expect, it } from 'vitest'

import {
  adminAffiliateTeamSummaryRoute,
  buildAdminAffiliateTeamQuery,
  parseAdminAffiliateTeamQuery
} from '../useAdminAffiliateTeamQuery'

const today = new Date(2026, 9, 23)

describe('useAdminAffiliateTeamQuery', () => {
  it('falls back to defaults for missing or invalid values and omits defaults when building', () => {
    const state = parseAdminAffiliateTeamQuery({ level: 'vip', sort: 'cost', order: 'up', page: '0', size: '7', q: '  ' }, today)
    expect(state).toEqual({
      start: '2026-10-17',
      end: '2026-10-23',
      level: '',
      q: '',
      sort: 'team_cost',
      order: 'desc',
      page: 1,
      size: 20
    })
    expect(buildAdminAffiliateTeamQuery(state, today)).toEqual({})
  })

  it('round-trips every filter', () => {
    const query = {
      start: '2026-10-01',
      end: '2026-10-07',
      level: 'downgraded',
      q: 'wang',
      sort: 'earnings',
      order: 'asc',
      page: '3',
      size: '50'
    }
    expect(buildAdminAffiliateTeamQuery(parseAdminAffiliateTeamQuery(query, today), today)).toEqual(query)
  })

  it('clamps a range longer than 90 days to end - 89', () => {
    const state = parseAdminAffiliateTeamQuery({ start: '2026-01-01', end: '2026-10-07' }, today)
    expect(state.start).toBe('2026-07-10')
    expect(state.end).toBe('2026-10-07')
  })

  it('returns to the summary with the given range when nothing was remembered', () => {
    expect(adminAffiliateTeamSummaryRoute({ start: '2026-10-01', end: '2026-10-07' })).toEqual({
      name: 'AdminAffiliateTeam',
      query: { start: '2026-10-01', end: '2026-10-07' }
    })
  })
})
