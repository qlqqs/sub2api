import { describe, expect, it, vi } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import {
  buildAffiliateTeamQuery,
  clampAffiliateTeamRange,
  defaultAffiliateTeamRange,
  parseAffiliateTeamQuery,
  useAffiliateTeamQuery,
  type AffiliateTeamView
} from '../useAffiliateTeamQuery'

const today = new Date(2026, 9, 22, 12, 0, 0)

describe('parseAffiliateTeamQuery', () => {
  it('falls back to defaults when the query is empty', () => {
    expect(parseAffiliateTeamQuery({}, 'team', today)).toEqual({
      start: '2026-10-16',
      end: '2026-10-22',
      tab: 'customers',
      sort: 'cost',
      order: 'desc',
      page: 1,
      size: 20,
      q: '',
      model: '',
      group: null
    })
  })

  it('keeps whitelisted values', () => {
    const state = parseAffiliateTeamQuery(
      { start: '2026-10-01', end: '2026-10-10', tab: 'agents', sort: 'earnings', order: 'asc', page: '3', size: '50' },
      'team',
      today
    )
    expect(state).toMatchObject({ start: '2026-10-01', end: '2026-10-10', tab: 'agents', sort: 'earnings', order: 'asc', page: 3, size: 50 })
  })

  it('rejects invalid values instead of passing them through', () => {
    const state = parseAffiliateTeamQuery(
      {
        start: '2026-02-30',
        end: 'yesterday',
        tab: 'admins',
        sort: 'balance',
        order: 'up',
        page: '-2',
        size: '33',
        group: '1'
      },
      'team',
      today
    )
    expect(state).toEqual(parseAffiliateTeamQuery({}, 'team', today))
  })

  it('validates sort keys against the active tab', () => {
    expect(parseAffiliateTeamQuery({ tab: 'agents', sort: 'cost' }, 'team', today).sort).toBe('team_cost')
    expect(parseAffiliateTeamQuery({ sort: 'team_cost' }, 'team', today).sort).toBe('cost')
    expect(parseAffiliateTeamQuery({ sort: 'last_used_at' }, 'agent', today).sort).toBe('last_used_at')
  })

  it('only allows the agents tab on the team overview', () => {
    expect(parseAffiliateTeamQuery({ tab: 'agents' }, 'agent', today).tab).toBe('customers')
    expect(parseAffiliateTeamQuery({ tab: 'agents' }, 'customer', today).tab).toBe('customers')
  })

  it('treats a reversed range as invalid and clamps ranges longer than 90 days', () => {
    expect(parseAffiliateTeamQuery({ start: '2026-10-10', end: '2026-10-01' }, 'team', today)).toMatchObject(defaultAffiliateTeamRange(today))
    expect(parseAffiliateTeamQuery({ start: '2026-01-01', end: '2026-10-22' }, 'team', today)).toMatchObject({
      start: '2026-07-25',
      end: '2026-10-22'
    })
    expect(clampAffiliateTeamRange('2026-07-25', '2026-10-22').clamped).toBe(false)
    expect(clampAffiliateTeamRange('2026-07-24', '2026-10-22')).toEqual({ start: '2026-07-25', end: '2026-10-22', clamped: true })
  })

  it('scopes search to customer lists and model/group filters to the customer view', () => {
    const raw = { q: '  alice ', model: 'gpt-5', group: '0' }
    expect(parseAffiliateTeamQuery(raw, 'team', today)).toMatchObject({ q: 'alice', model: '', group: null })
    expect(parseAffiliateTeamQuery({ ...raw, tab: 'agents' }, 'team', today).q).toBe('')
    expect(parseAffiliateTeamQuery(raw, 'customer', today)).toMatchObject({ q: '', model: 'gpt-5', group: 0 })
    expect(parseAffiliateTeamQuery({ group: '-1' }, 'customer', today).group).toBeNull()
    expect(parseAffiliateTeamQuery({ group: 'x' }, 'customer', today).group).toBeNull()
  })

  it('uses the first value of repeated keys', () => {
    expect(parseAffiliateTeamQuery({ page: ['2', '5'] }, 'team', today).page).toBe(2)
  })
})

describe('buildAffiliateTeamQuery', () => {
  it('omits defaults so the canonical URL stays clean', () => {
    expect(buildAffiliateTeamQuery(parseAffiliateTeamQuery({}, 'team', today), 'team', today)).toEqual({})
  })

  it('writes only keys relevant to the view', () => {
    const state = {
      ...parseAffiliateTeamQuery({}, 'customer', today),
      start: '2026-10-01',
      model: 'claude',
      group: 0,
      page: 2,
      sort: 'requests' as const,
      q: 'ignored'
    }
    expect(buildAffiliateTeamQuery(state, 'customer', today)).toEqual({
      start: '2026-10-01',
      end: '2026-10-22',
      model: 'claude',
      group: '0',
      page: '2'
    })
  })
})

describe('useAffiliateTeamQuery', () => {
  let api!: ReturnType<typeof useAffiliateTeamQuery>
  const Host = defineComponent({
    props: { view: { type: String, required: true } },
    setup(props) {
      api = useAffiliateTeamQuery(() => props.view as AffiliateTeamView)
      return () => h('div')
    }
  })

  async function setup(path: string) {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/affiliate/team', name: 'AffiliateTeam', component: Host },
        { path: '/affiliate/team/customers/:id', name: 'AffiliateTeamCustomer', component: Host }
      ]
    })
    await router.push(path)
    await router.isReady()
    mount(Host, { props: { view: 'team' }, global: { plugins: [router] } })
    return { router, api }
  }

  it('drill-down links carry only the date range', async () => {
    const { api } = await setup('/affiliate/team?start=2026-10-01&end=2026-10-07&tab=agents&sort=earnings&order=asc&page=4&q=bob')
    expect(api.state.value).toMatchObject({ tab: 'agents', sort: 'earnings', page: 4 })
    expect(api.drillDownQuery.value).toEqual({ start: '2026-10-01', end: '2026-10-07' })
  })

  it('replaces the current entry for in-page changes so Back returns to the previous level', async () => {
    const { router, api } = await setup('/affiliate/team/customers/9')
    await router.push('/affiliate/team?page=2')

    await api.update({ sort: 'requests', page: 1 })
    await nextTick()
    expect(router.currentRoute.value.fullPath).toBe('/affiliate/team?sort=requests')

    router.back()
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(router.currentRoute.value.fullPath).toBe('/affiliate/team/customers/9')
  })

  it('skips navigation when nothing changes', async () => {
    const { router, api } = await setup('/affiliate/team?sort=requests')
    const replace = vi.spyOn(router, 'replace')
    await api.update({ sort: 'requests' })
    expect(replace).not.toHaveBeenCalled()
  })
})
