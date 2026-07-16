import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import AccountBalanceCell from '../AccountBalanceCell.vue'
import type { AccountBalanceRowState, UpstreamBalanceResult } from '@/types'

const messages: Record<string, string> = {
  'admin.accounts.upstreamBalance.notApplicable': 'Not applicable',
  'admin.accounts.upstreamBalance.notQueried': 'Not queried',
  'admin.accounts.upstreamBalance.loading': 'Loading',
  'admin.accounts.upstreamBalance.unsupported': 'Unsupported',
  'admin.accounts.upstreamBalance.refresh': 'Refresh',
  'admin.accounts.upstreamBalance.apiKeyRate': 'Key rate',
  'admin.accounts.upstreamBalance.queriedAt': 'Queried at',
  'admin.accounts.upstreamBalance.scope.user': 'User balance',
  'admin.accounts.upstreamBalance.scope.apiKey': 'Key quota',
  'admin.accounts.upstreamBalance.scope.unknown': 'Available quota',
  'admin.accounts.upstreamBalance.errors.fallback': 'Query failed'
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: ref('en-US'),
    t: (key: string) => messages[key] ?? key
  })
}))

const successfulResult: UpstreamBalanceResult = {
  status: 'available',
  platform_type: 'sub2api',
  scope: 'api_key',
  remaining: '0',
  unit: 'USD',
  api_key_rate: '1.5',
  queried_at: '2026-07-16T12:00:00Z'
}

function mountBalanceCell(state: AccountBalanceRowState, accountType: 'upstream' | 'oauth' = 'upstream') {
  return mount(AccountBalanceCell, {
    props: {
      accountType,
      state
    }
  })
}

describe('AccountBalanceCell', () => {
  it('renders non-upstream accounts as not applicable without a refresh action', () => {
    const wrapper = mountBalanceCell({ phase: 'idle' }, 'oauth')

    expect(wrapper.text()).toContain('Not applicable')
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('renders zero balance, scope, optional rate, and query time', () => {
    const wrapper = mountBalanceCell({
      phase: 'available',
      latest_result: successfulResult,
      last_successful_result: successfulResult
    })

    expect(wrapper.text()).toContain('Key quota: $0.00')
    expect(wrapper.text()).toContain('Key rate: 1.5x')
    expect(wrapper.text()).not.toContain('Queried at:')
    const queriedAtIcon = wrapper.get('[aria-label^="Queried at:"]')
    const refreshButton = wrapper.get('button[aria-label="Refresh"]')
    expect(queriedAtIcon.attributes('title')).toBe(queriedAtIcon.attributes('aria-label'))
    expect(refreshButton.element.compareDocumentPosition(queriedAtIcon.element)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING
    )
  })

  it.each([
    ['12.345', 'USD', '$12.35'],
    ['-0.004', 'USD', '$0.00'],
    ['-0.005', 'USD', '$-0.01'],
    ['9007199254740993.129', 'USD', '$9007199254740993.13'],
    ['.5', 'credits', '0.50 credits'],
    ['not-a-number', 'USD', '$not-a-number']
  ])('formats balance %s %s as %s', (remaining, unit, expectedBalance) => {
    const wrapper = mountBalanceCell({
      phase: 'available',
      latest_result: {
        ...successfulResult,
        remaining,
        unit
      }
    })

    expect(wrapper.text()).toContain(`Key quota: ${expectedBalance}`)
  })

  it.each([
    ['user', 'User balance'],
    ['unknown', 'Available quota']
  ] as const)('renders the %s scope without an absent rate', (scope, expectedLabel) => {
    const wrapper = mountBalanceCell({
      phase: 'available',
      latest_result: {
        ...successfulResult,
        scope,
        remaining: '8',
        api_key_rate: undefined
      }
    })

    expect(wrapper.text()).toContain(`${expectedLabel}: $8.00`)
    expect(wrapper.text()).not.toContain('Key rate:')
  })

  it('renders explicit idle and loading status text', async () => {
    const wrapper = mountBalanceCell({ phase: 'idle' })
    expect(wrapper.text()).toContain('Not queried')

    await wrapper.setProps({ state: { phase: 'loading' } })
    expect(wrapper.text()).toContain('Loading')
  })

  it('keeps the last successful result visible when a refresh fails', () => {
    const wrapper = mountBalanceCell({
      phase: 'failed',
      last_successful_result: successfulResult,
      error_message: 'Credentials rejected'
    })

    expect(wrapper.text()).toContain('Key quota: $0.00')
    expect(wrapper.text()).toContain('Credentials rejected')
  })

  it('does not present stale balance as current when the protocol is unsupported', () => {
    const wrapper = mountBalanceCell({
      phase: 'unsupported',
      last_successful_result: successfulResult
    })

    expect(wrapper.text()).toContain('Unsupported')
    expect(wrapper.text()).not.toContain('Key quota: $0.00')
  })

  it('disables refresh while loading and emits refresh otherwise', async () => {
    const wrapper = mountBalanceCell({ phase: 'idle' })
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)

    await wrapper.setProps({ state: { phase: 'loading' } })
    const refreshButton = wrapper.get('button')
    expect(refreshButton.attributes('disabled')).toBeDefined()
    expect(refreshButton.attributes('aria-label')).toBe('Loading')
    expect(refreshButton.attributes('aria-busy')).toBe('true')
    expect(refreshButton.get('svg').classes()).toContain('animate-spin')
  })
})
