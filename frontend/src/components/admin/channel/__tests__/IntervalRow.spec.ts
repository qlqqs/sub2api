import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import IntervalRow from '../IntervalRow.vue'
import type { IntervalFormEntry } from '../types'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
const interval: IntervalFormEntry = {
  min_tokens: 0, max_tokens: null, tier_label: '', sort_order: 0,
  input_price: null, output_price: null, cache_write_price: null, cache_read_price: null,
  input_multiplier: null, output_multiplier: null, cache_write_multiplier: null,
  cache_read_multiplier: null, per_request_price: null,
}

describe('pricing interval token bounds', () => {
  it.each([
    ['1e6', 1000000, 1000000],
    ['2.72e5', 272000, 272000],
    ['128000', 128000, 128000],
    ['', 0, null],
  ])('parses %s without losing the exponent', async (value, min, max) => {
    const wrapper = mount(IntervalRow, { props: { interval, mode: 'token' } })
    const inputs = wrapper.findAll('input[type="number"]')
    await inputs[0].setValue(value)
    await inputs[1].setValue(value)
    expect(wrapper.emitted('update')?.[0]?.[0]).toMatchObject({ min_tokens: min })
    expect(wrapper.emitted('update')?.[1]?.[0]).toMatchObject({ max_tokens: max })
  })
})

describe('video tier select', () => {
  it('lists every video tier with readable names and emits the raw tier_label', async () => {
    const wrapper = mount(IntervalRow, { props: { interval: { ...interval, tier_label: '720p' }, mode: 'video' } })
    const select = wrapper.get('[data-testid="video-tier-select"]')
    expect(select.findAll('option').map((o) => [o.attributes('value'), o.text()])).toEqual([
      ['480p', '480p'], ['720p', '720p'], ['1080p', '1080p'], ['4k', '4k'],
      ['480p-ref', 'admin.channels.form.videoTierRef'], ['720p-ref', 'admin.channels.form.videoTierRef'],
      ['1080p-ref', 'admin.channels.form.videoTierRef'], ['4k-ref', 'admin.channels.form.videoTierRef'],
    ])
    await select.setValue('1080p-ref')
    expect(wrapper.emitted('update')?.[0]?.[0]).toMatchObject({ tier_label: '1080p-ref' })
  })

  it('keeps an existing label that is not in the list', () => {
    const wrapper = mount(IntervalRow, { props: { interval: { ...interval, tier_label: '2k' }, mode: 'video' } })
    const options = wrapper.findAll('[data-testid="video-tier-select"] option').map((o) => o.attributes('value'))
    expect(options.at(-1)).toBe('2k')
    expect((wrapper.get('[data-testid="video-tier-select"]').element as HTMLSelectElement).value).toBe('2k')
  })
})
