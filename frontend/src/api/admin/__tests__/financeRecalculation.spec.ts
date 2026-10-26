import { beforeEach, describe, expect, it, vi } from 'vitest'
import { financeRecalculationAPI as api } from '../financeRecalculation'
const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))
beforeEach(() => { vi.resetAllMocks(); client.get.mockResolvedValue({ data: {} }); client.post.mockResolvedValue({ data: {} }) })
describe('finance recalculation protocol', () => {
  it('posts only the explicit preview and confirmation contracts', async () => {
    const payload = { kind: 'payment_fee' as const, start: '2026-10-01', end: '2026-10-26', provider_id: 3, idempotency_key: 'key' }
    await api.create(payload)
    expect(client.post).toHaveBeenLastCalledWith('/admin/finance/recalculations', payload)
    await api.confirm(9, 'fingerprint')
    expect(client.post).toHaveBeenLastCalledWith('/admin/finance/recalculations/9/confirm', { config_fingerprint: 'fingerprint', acknowledged: true })
    await api.retrySync(9)
    expect(client.post).toHaveBeenLastCalledWith('/admin/finance/recalculations/9/retry-sync')
  })
  it('whitelists the optional usage baseline flag and strips unrelated or unknown fields', async () => {
    const payload = { kind: 'usage_cost' as const, start: '2026-10-01', end: '2026-10-26', account_id: 11, model: 'model', accept_retained_baseline: true, provider_id: 3, idempotency_key: 'baseline-key', credentials: 'NEVER_SEND' }
    await api.create(payload)
    expect(client.post).toHaveBeenLastCalledWith('/admin/finance/recalculations', {
      kind: 'usage_cost', start: payload.start, end: payload.end, account_id: 11, model: 'model', accept_retained_baseline: true, idempotency_key: 'baseline-key'
    })
    await api.create({ kind: 'usage_cost', start: payload.start, end: payload.end, accept_retained_baseline: false, idempotency_key: 'unaccepted' })
    expect(client.post.mock.lastCall?.[1].accept_retained_baseline).toBe(false)
    await api.create({ kind: 'usage_cost', start: payload.start, end: payload.end, idempotency_key: 'default' })
    expect(client.post.mock.lastCall?.[1]).not.toHaveProperty('accept_retained_baseline')
    await api.create({ ...payload, kind: 'payment_fee' })
    expect(client.post).toHaveBeenLastCalledWith('/admin/finance/recalculations', {
      kind: 'payment_fee', start: payload.start, end: payload.end, provider_id: 3, idempotency_key: 'baseline-key'
    })
  })
  it('preserves exact decimal strings and nulls and forwards cancellation/pagination', async () => {
    const data = { items: [{ id: 1, old_cost: null, new_cost: '0.00000001', delta: null }], total: 1 }
    client.get.mockResolvedValue({ data })
    const signal = new AbortController().signal
    const params = { page: 2, page_size: 20 }
    expect(await api.items(9, params, signal)).toBe(data)
    expect(client.get).toHaveBeenLastCalledWith('/admin/finance/recalculations/9/items', { params, signal })
    await api.list(params, signal)
    expect(client.get).toHaveBeenLastCalledWith('/admin/finance/recalculations', { params, signal })
    await api.get(9, signal)
    expect(client.get).toHaveBeenLastCalledWith('/admin/finance/recalculations/9', { signal })
  })
})
