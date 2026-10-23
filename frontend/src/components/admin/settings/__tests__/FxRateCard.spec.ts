import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import FxRateCard from '../FxRateCard.vue'
import type { FxRateView } from '@/api/admin/fxRate'

const api = vi.hoisted(() => ({
  get: vi.fn(),
  update: vi.fn(),
  refresh: vi.fn()
}))
const toast = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn(), fetchPublicSettings: vi.fn() }))

vi.mock('@/api', () => ({ adminAPI: { fxRate: api } }))
vi.mock('@/stores', () => ({ useAppStore: () => toast }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key}|${JSON.stringify(params)}` : key)
    })
  }
})

function view(overrides: Partial<FxRateView> = {}): FxRateView {
  return {
    auto_enabled: true,
    default_rate: 6.8,
    effective_rate: 7.12,
    effective_from: 'fetched',
    fetched_rate: 7.12,
    fetched_at: '2026-10-23T02:00:00Z',
    source: 'api.frankfurter.dev',
    stale: false,
    last_error: '',
    last_attempt_at: '2026-10-23T02:00:00Z',
    ...overrides
  }
}

async function mountCard() {
  const wrapper = mount(FxRateCard, { global: { stubs: { Icon: true } } })
  await flushPromises()
  return wrapper
}

describe('FxRateCard (CUSTOM)', () => {
  beforeEach(() => {
    api.get.mockReset().mockResolvedValue(view())
    api.update.mockReset()
    api.refresh.mockReset()
    toast.showSuccess.mockReset()
    toast.showError.mockReset()
    toast.fetchPublicSettings.mockReset().mockResolvedValue(null)
  })

  it('shows the effective rate, its origin, source and fetch time', async () => {
    const wrapper = await mountCard()
    expect(wrapper.get('[data-testid="fx-rate-effective"]').text()).toContain('$1 = ¥7.12')
    expect(wrapper.get('[data-testid="fx-rate-effective"]').text()).toContain('admin.settings.fxRate.fromFetched')
    const fetched = wrapper.get('[data-testid="fx-rate-fetched"]').text()
    expect(fetched).toContain('7.12')
    expect(fetched).toContain('api.frankfurter.dev')
    expect(wrapper.find('[data-testid="fx-rate-stale"]').exists()).toBe(false)
    expect(wrapper.get<HTMLInputElement>('[data-testid="fx-rate-default-input"]').element.value).toBe('6.8')
    // 未改动时保存按钮禁用
    expect(wrapper.get('[data-testid="fx-rate-save"]').attributes('disabled')).toBeDefined()
  })

  it('warns when the rate has never been fetched and shows the last failure', async () => {
    api.get.mockResolvedValue(
      view({ stale: true, effective_rate: 6.8, effective_from: 'default', fetched_rate: null, fetched_at: null, source: '', last_error: 'all sources failed' })
    )
    const wrapper = await mountCard()
    expect(wrapper.get('[data-testid="fx-rate-stale"]').text()).toContain('admin.settings.fxRate.staleNever')
    expect(wrapper.get('[data-testid="fx-rate-stale"]').text()).toContain('"rate":"6.8"')
    expect(wrapper.get('[data-testid="fx-rate-effective"]').text()).toContain('admin.settings.fxRate.fromDefault')
    expect(wrapper.get('[data-testid="fx-rate-fetched"]').text()).toContain('admin.settings.fxRate.neverFetched')
    expect(wrapper.get('[data-testid="fx-rate-last-error"]').text()).toContain('all sources failed')
  })

  it('warns when the last successful fetch is more than 3 days old', async () => {
    api.get.mockResolvedValue(view({ stale: true, fetched_at: '2026-10-18T02:00:00Z' }))
    const wrapper = await mountCard()
    const stale = wrapper.get('[data-testid="fx-rate-stale"]').text()
    expect(stale).toContain('admin.settings.fxRate.stale|')
    expect(stale).toContain('"rate":"7.12"')
  })

  it('saves the auto-update switch and default rate', async () => {
    api.update.mockResolvedValue(view({ auto_enabled: false, default_rate: 7.1, effective_rate: 7.1, effective_from: 'default' }))
    const wrapper = await mountCard()
    await wrapper.get('[role="switch"]').trigger('click')
    await wrapper.get('[data-testid="fx-rate-default-input"]').setValue('7.1')
    await wrapper.get('[data-testid="fx-rate-save"]').trigger('click')
    await flushPromises()
    expect(api.update).toHaveBeenCalledWith({ auto_enabled: false, default_rate: 7.1 })
    expect(toast.showSuccess).toHaveBeenCalledWith('admin.settings.fxRate.saved')
    expect(wrapper.get('[data-testid="fx-rate-effective"]').text()).toContain('$1 = ¥7.1')
    // 公开设置缓存随之刷新(本会话的折扣按新汇率)
    expect(toast.fetchPublicSettings).toHaveBeenCalledWith(true)
  })

  it.each(['4.99', '9.01', ''])('rejects default rate %s outside 5–9', async (value) => {
    const wrapper = await mountCard()
    await wrapper.get('[data-testid="fx-rate-default-input"]').setValue(value)
    expect(wrapper.get('[data-testid="fx-rate-default-error"]').text()).toContain('admin.settings.fxRate.defaultRateInvalid')
    expect(wrapper.get('[data-testid="fx-rate-save"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="fx-rate-save"]').trigger('click')
    expect(api.update).not.toHaveBeenCalled()
  })

  it('updates now and shows the new value', async () => {
    api.refresh.mockResolvedValue({ ok: true, error: '', rate: view({ effective_rate: 7.2, fetched_rate: 7.2 }) })
    const wrapper = await mountCard()
    await wrapper.get('[data-testid="fx-rate-refresh"]').trigger('click')
    await flushPromises()
    expect(toast.showSuccess).toHaveBeenCalledWith('admin.settings.fxRate.refreshed|{"rate":"7.2"}')
    expect(wrapper.get('[data-testid="fx-rate-effective"]').text()).toContain('$1 = ¥7.2')
    expect(toast.fetchPublicSettings).toHaveBeenCalledWith(true)
  })

  it('reports the fetched value (not the default in effect) when auto update is off', async () => {
    api.get.mockResolvedValue(view({ auto_enabled: false, effective_rate: 6.8, effective_from: 'default' }))
    api.refresh.mockResolvedValue({
      ok: true,
      error: '',
      rate: view({ auto_enabled: false, effective_rate: 6.8, effective_from: 'default', fetched_rate: 7.05 })
    })
    const wrapper = await mountCard()
    await wrapper.get('[data-testid="fx-rate-refresh"]').trigger('click')
    await flushPromises()
    expect(toast.showSuccess).toHaveBeenCalledWith('admin.settings.fxRate.refreshed|{"rate":"7.05"}')
    expect(wrapper.get('[data-testid="fx-rate-effective"]').text()).toContain('admin.settings.fxRate.fromDefault')
  })

  it('shows the failure reason when every source fails (ok=false)', async () => {
    api.refresh.mockResolvedValue({
      ok: false,
      error: 'all sources failed',
      rate: view({ stale: true, last_error: 'all sources failed' })
    })
    const wrapper = await mountCard()
    await wrapper.get('[data-testid="fx-rate-default-input"]').setValue('7.3')
    await wrapper.get('[data-testid="fx-rate-refresh"]').trigger('click')
    await flushPromises()
    expect(toast.showError).toHaveBeenCalledWith('admin.settings.fxRate.refreshFailed|{"error":"all sources failed"}')
    expect(toast.fetchPublicSettings).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="fx-rate-stale"]').exists()).toBe(true)
    // 未保存的输入保留
    expect(wrapper.get<HTMLInputElement>('[data-testid="fx-rate-default-input"]').element.value).toBe('7.3')
  })

  it('degrades gracefully when the backend does not support the endpoint', async () => {
    api.get.mockRejectedValue({ message: 'not found' })
    const wrapper = await mountCard()
    expect(wrapper.get('[data-testid="fx-rate-load-error"]').text()).toContain('admin.settings.fxRate.loadFailed')
    api.get.mockResolvedValue(view())
    await wrapper.get('[data-testid="fx-rate-load-error"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="fx-rate-effective"]').exists()).toBe(true)
  })
})
