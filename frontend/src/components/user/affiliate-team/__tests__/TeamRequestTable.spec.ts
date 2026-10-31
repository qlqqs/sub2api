import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { AffiliateTeamRequest } from '@/types'
import TeamRequestTable from '../TeamRequestTable.vue'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: unknown) => (params === undefined ? key : `${key}${JSON.stringify(params)}`),
      locale: { value: 'zh' }
    })
  }
})

function request(overrides: Partial<AffiliateTeamRequest>): AffiliateTeamRequest {
  return {
    created_at: '2026-10-31T08:00:00Z',
    model: 'seedance-2.0',
    group_name: 'Seedance 视频',
    billing_mode: 'video',
    input_tokens: 0,
    output_tokens: 0,
    cache_creation_tokens: 0,
    cache_read_tokens: 0,
    image_count: 0,
    video_count: 1,
    cost: 3.75,
    rebate_factor: 100,
    ...overrides
  }
}

describe('TeamRequestTable', () => {
  it('shows the rebate factor badge only when the factor is not 100', () => {
    const wrapper = mount(TeamRequestTable, {
      props: {
        rows: [
          request({ rebate_factor: 30 }),
          request({ rebate_factor: 12.5, group_name: '' }),
          request({ rebate_factor: 0 }),
          request({ rebate_factor: 100 })
        ]
      },
      global: { stubs: { Icon: true } }
    })

    const badges = wrapper.findAll('[data-test="team-request-rebate-factor"]').map((badge) => badge.text())
    expect(badges).toEqual([
      'affiliate.team.rebateFactor{"rate":"30"}',
      'affiliate.team.rebateFactor{"rate":"12.5"}',
      'affiliate.team.rebateFactor{"rate":"0"}'
    ])
  })
})
