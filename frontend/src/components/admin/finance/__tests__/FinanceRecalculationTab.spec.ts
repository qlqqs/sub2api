import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import FinanceRecalculationTab from '../FinanceRecalculationTab.vue'
import type { RecalculationBatch } from '@/api/admin/financeRecalculation'
import en from '@/i18n/locales/en/admin/financeRecalculation'
import zh from '@/i18n/locales/zh/admin/financeRecalculation'

vi.mock('vue-i18n', async importOriginal => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const { default: messages } = await import('@/i18n/locales/en/admin/financeRecalculation')
  const { ref } = await import('vue')
  const lookup = (key: string) => key.replace(/^admin\./, '').split('.').reduce<unknown>((node, part) => node && typeof node === 'object' ? (node as Record<string, unknown>)[part] : undefined, messages)
  return { ...actual, useI18n: () => ({ t: (key: string) => lookup(key) ?? key, te: (key: string) => typeof lookup(key) === 'string', locale: ref('en') }) }
})

const mocks = vi.hoisted(() => ({
  api: { create: vi.fn(), list: vi.fn(), get: vi.fn(), items: vi.fn(), confirm: vi.fn(), retrySync: vi.fn(), cancel: vi.fn() },
  getSettings: vi.fn(), getProviders: vi.fn(), getModels: vi.fn(), user: { id: 7 }
}))
vi.mock('@/api/admin/financeRecalculation', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/admin/financeRecalculation')>(), financeRecalculationAPI: mocks.api
}))
vi.mock('@/api/admin/finance', () => ({ getSettings: mocks.getSettings }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { getProviders: mocks.getProviders } }))
vi.mock('@/api/admin/accounts', () => ({ getAvailableModels: mocks.getModels }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: mocks.user }) }))

const L = en.financeRecalculation
function batch(overrides: Partial<RecalculationBatch> = {}): RecalculationBatch {
  return { id: 1, kind: 'usage_cost', start: '2026-10-01', end: '2026-10-26', status: 'ready', scanned: 3, changed: 1, unchanged: 1, skipped: 1, old_total: '1.00000000', new_total: '2.00000000', delta: '1.00000000', actor_id: 7, config_fingerprint: 'saved-fingerprint', cutoff_at: '2026-10-26T01:00:00Z', expires_at: '2026-10-27T01:00:00Z', created_at: '2026-10-26T01:00:00Z', error_code: '', error_message: '', snapshot_summary: ['Account model rule #42'], ...overrides }
}
const SelectStub = {
  props: ['modelValue', 'options'], emits: ['update:modelValue'],
  template: `<select :value="modelValue" @change="$emit('update:modelValue', $event.target.value === '' ? '' : isNaN(Number($event.target.value)) ? $event.target.value : Number($event.target.value))"><option v-for="o in options" :value="o.value" :key="o.value">{{ o.label }}</option></select>`
}
const TableStub = {
  props: ['data', 'columns', 'loading'],
  template: `<div><div v-for="row in data" :key="row.id" :data-row="row.id"><div v-for="c in columns" :key="c.key" :data-column="c.key"><slot :name="'cell-' + c.key" :row="row">{{ row[c.key] }}</slot></div></div></div>`
}
const wrappers: VueWrapper[] = []
function render(realDialog = false) {
  const wrapper = mount(FinanceRecalculationTab, {
    global: {
      stubs: {
        BaseDialog: realDialog ? false : { props: ['show', 'title'], emits: ['close'], template: '<div v-if="show"><h2>{{ title }}</h2><button data-testid="dialog-close" @click="$emit(\'close\')">Close dialog</button><slot /><slot name="footer" /></div>' },
        DataTable: TableStub, Select: SelectStub, Pagination: true,
        DateRangePicker: { props: ['startDate', 'endDate'], emits: ['update:startDate', 'update:endDate'], template: `<div><input data-testid="start" :value="startDate" @input="$emit('update:startDate', $event.target.value)" /><input data-testid="end" :value="endDate" @input="$emit('update:endDate', $event.target.value)" /></div>` }
      }
    }
  })
  wrappers.push(wrapper)
  return wrapper
}
async function newPreview(w: VueWrapper) {
  await w.get('[data-testid="new-preview"]').trigger('click')
  await flushPromises()
  await w.get('form').trigger('submit')
  await flushPromises()
}
async function open(w: VueWrapper, id = 1) {
  await flushPromises()
  await w.get(`[data-testid="batch-${id}"]`).trigger('click')
  await flushPromises()
}
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-10-26T02:00:00Z'))
  vi.resetAllMocks()
  mocks.user.id = 7
  mocks.api.list.mockResolvedValue({ items: [batch()], total: 1 })
  mocks.api.get.mockResolvedValue(batch())
  mocks.api.create.mockResolvedValue(batch())
  mocks.api.confirm.mockResolvedValue(batch({ status: 'queued' }))
  mocks.api.items.mockResolvedValue({ items: [{ id: 10, status: 'skipped', reason: 'missing_image_count', source: 'custom_image', old_cost: null, new_cost: null, delta: null, applied: false }], total: 1 })
  mocks.getSettings.mockResolvedValue({ accounts: [{ id: 11, name: 'Account', deleted: false }] })
  mocks.getProviders.mockResolvedValue({ data: [{ id: 12, name: 'Provider', config: { secret: 'NEVER_RENDER' } }] })
  mocks.getModels.mockResolvedValue([{ id: 'historical-model' }])
})
afterEach(() => {
  wrappers.splice(0).forEach(w => w.unmount())
  vi.useRealTimers()
})

describe('historical recalculation', () => {
  it('creates a preview only, shows plain results and requires a separate irrevocable acknowledgement', async () => {
    const w = render()
    await newPreview(w)
    expect(mocks.api.create).toHaveBeenCalledWith(expect.objectContaining({ kind: 'usage_cost', start: '2026-10-01', end: '2026-10-26', idempotency_key: expect.any(String) }))
    expect(mocks.api.confirm).not.toHaveBeenCalled()
    // Internal fingerprints and English server summaries are not shown to admins.
    expect(w.text()).not.toContain('saved-fingerprint')
    expect(w.text()).not.toContain('Account model rule #42')
    expect(w.text()).toContain(L.units.usage_cost)
    expect(w.text()).not.toContain('NEVER_RENDER')
    await w.get('[data-testid="review-confirm"]').trigger('click')
    expect(w.text()).toContain(L.irreversible)
    expect(w.text()).toContain(L.confirmHint)
    expect(w.get('[data-testid="confirm-execute"]').attributes('disabled')).toBeDefined()
    await w.get('[data-testid="acknowledge"]').setValue(true)
    // Future reads retain the durable queued state.
    mocks.api.get.mockResolvedValue(batch({ status: 'queued' }))
    await w.get('[data-testid="confirm-execute"]').trigger('click')
    await flushPromises()
    expect(mocks.api.confirm).toHaveBeenCalledTimes(1)
    expect(mocks.api.confirm).toHaveBeenCalledWith(1, 'saved-fingerprint')
    expect(w.text()).toContain(L.syncWarning)
    expect(w.find('[data-testid="cancel-preview"]').exists()).toBe(false)
    expect(w.emitted('changed')).toBeUndefined()
  })

  it('defaults baseline acceptance off and still permits an unaccepted preview', async () => {
    const w = render()
    await w.get('[data-testid="new-preview"]').trigger('click'); await flushPromises()
    expect((w.get('[data-testid="accept-retained-baseline"]').element as HTMLInputElement).checked).toBe(false)
    expect(w.text()).toContain(L.baseline.hint)
    expect(w.text()).toContain(L.boundaries)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.api.create).toHaveBeenCalledWith(expect.objectContaining({ accept_retained_baseline: false }))
    expect(w.find('[data-testid="retained-baseline-summary"]').exists()).toBe(false)
    expect(mocks.api.confirm).not.toHaveBeenCalled()
  })

  it('shows persisted acceptance and the reconciliation condition in preview, confirmation and reopened details', async () => {
    const saved = batch({ accept_retained_baseline: true })
    mocks.api.create.mockResolvedValue(saved)
    mocks.api.get.mockResolvedValue(saved)
    const w = render()
    await w.get('[data-testid="new-preview"]').trigger('click'); await flushPromises()
    await w.get('[data-testid="accept-retained-baseline"]').setValue(true)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.api.create).toHaveBeenCalledWith(expect.objectContaining({ kind: 'usage_cost', accept_retained_baseline: true }))
    expect(w.get('[data-testid="retained-baseline-summary"]').text()).toContain(L.baseline.accepted)
    await w.get('[data-testid="review-confirm"]').trigger('click')
    expect(w.get('[data-testid="retained-baseline-summary"]').text()).toContain(L.baseline.condition)
    expect(w.text()).toContain(L.confirmHint)
    expect(w.get('[data-testid="confirm-execute"]').attributes('disabled')).toBeDefined()
    expect(mocks.api.confirm).not.toHaveBeenCalled()
    await w.get('[data-testid="dialog-close"]').trigger('click')
    await w.get('[data-testid="new-preview"]').trigger('click'); await flushPromises()
    expect((w.get('[data-testid="accept-retained-baseline"]').element as HTMLInputElement).checked).toBe(false)
    await w.get('[data-testid="dialog-close"]').trigger('click'); await open(w)
    expect(w.get('[data-testid="retained-baseline-summary"]').text()).toContain(L.baseline.accepted)
    await w.get('[data-testid="review-confirm"]').trigger('click')
    await w.get('[data-testid="acknowledge"]').setValue(true)
    mocks.api.get.mockResolvedValue(batch({ status: 'queued', accept_retained_baseline: true }))
    await w.get('[data-testid="confirm-execute"]').trigger('click'); await flushPromises()
    expect(mocks.api.confirm).toHaveBeenCalledTimes(1)
    expect(mocks.api.confirm).toHaveBeenCalledWith(1, 'saved-fingerprint')
    expect(w.emitted('changed')).toBeUndefined()
  })

  it('changes the idempotency key when baseline acceptance changes but reuses it on identical retries', async () => {
    mocks.api.create.mockRejectedValue({ message: 'connection lost' })
    const w = render(); await newPreview(w)
    const originalKey = mocks.api.create.mock.calls[0][0].idempotency_key
    await w.get('[data-testid="accept-retained-baseline"]').setValue(true)
    await w.get('form').trigger('submit'); await flushPromises()
    const accepted = mocks.api.create.mock.calls[1][0]
    expect(accepted.accept_retained_baseline).toBe(true)
    expect(accepted.idempotency_key).not.toBe(originalKey)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.api.create.mock.calls[2][0].idempotency_key).toBe(accepted.idempotency_key)
  })

  it('resets acceptance on type switches and never displays it for a payment batch', async () => {
    mocks.api.create.mockResolvedValue(batch({ kind: 'payment_fee', accept_retained_baseline: true }))
    const w = render()
    await w.get('[data-testid="new-preview"]').trigger('click'); await flushPromises()
    await w.get('[data-testid="accept-retained-baseline"]').setValue(true)
    await w.get('#recalculation-kind').setValue('payment_fee')
    expect(w.find('[data-testid="accept-retained-baseline"]').exists()).toBe(false)
    await w.get('#recalculation-kind').setValue('usage_cost')
    expect((w.get('[data-testid="accept-retained-baseline"]').element as HTMLInputElement).checked).toBe(false)
    await w.get('[data-testid="accept-retained-baseline"]').setValue(true)
    await w.get('#recalculation-kind').setValue('payment_fee')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.api.create.mock.calls[0][0]).not.toHaveProperty('accept_retained_baseline')
    expect(w.find('[data-testid="retained-baseline-summary"]').exists()).toBe(false)
  })

  it('keeps decimal NULLs and skips visible instead of fabricating zero totals', async () => {
    mocks.api.get.mockResolvedValue(batch({ changed: 0, unchanged: 0, skipped: 3, old_total: '0', new_total: '0', delta: '0' }))
    const w = render(); await open(w)
    expect(w.text()).toContain(L.noComparableTotals)
    expect(w.text()).toContain(L.noChanges)
    expect(w.get('[data-row="10"] [data-column="reason"]').text()).toBe(L.reasons.missing_image_count)
    expect(w.get('[data-row="10"] [data-column="old_cost"]').text()).toBe('—')
    expect(w.get('[data-row="10"] [data-column="new_cost"]').text()).toBe('—')
    expect(w.find('[data-testid="review-confirm"]').exists()).toBe(false)
  })

  it('uses provider-only scope for fees and does not send hidden account/model filters', async () => {
    const w = render(); await w.get('[data-testid="new-preview"]').trigger('click'); await flushPromises()
    await w.get('#recalculation-account').setValue('11'); await flushPromises()
    await w.get('#recalculation-model').setValue('historical-model')
    await w.get('#recalculation-kind').setValue('payment_fee')
    await w.get('#recalculation-provider').setValue('12')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.api.create).toHaveBeenCalledWith({ kind: 'payment_fee', start: '2026-10-01', end: '2026-10-26', provider_id: 12, idempotency_key: expect.any(String) })
  })

  it('reuses the preview idempotency key after an uncertain failure and validates dates', async () => {
    mocks.api.create.mockRejectedValueOnce({ message: 'connection lost' })
    const w = render(); await newPreview(w)
    expect(w.text()).toContain('connection lost')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.api.create.mock.calls[0][0].idempotency_key).toBe(mocks.api.create.mock.calls[1][0].idempotency_key)
    await w.get('[data-testid="dialog-close"]').trigger('click')
    await w.get('[data-testid="new-preview"]').trigger('click'); await flushPromises()
    await w.get('[data-testid="start"]').setValue('2026-11-01')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.api.create).toHaveBeenCalledTimes(2)
    expect(w.text()).toContain(L.invalidRange)
  })

  it('reloads on 409, invalidates confirmation and requires a new preview', async () => {
    mocks.api.confirm.mockRejectedValue({ status: 409 })
    const w = render(); await open(w)
    await w.get('[data-testid="review-confirm"]').trigger('click')
    await w.get('[data-testid="acknowledge"]').setValue(true)
    await w.get('[data-testid="confirm-execute"]').trigger('click'); await flushPromises()
    expect(w.text()).toContain(L.conflictHint)
    expect(w.find('[data-testid="review-confirm"]').exists()).toBe(false)
    expect(w.find('[data-testid="preview-again"]').exists()).toBe(true)
    expect(mocks.api.get.mock.calls.length).toBeGreaterThan(1)
  })

  it('only lets the creator confirm and disallows expired previews', async () => {
    mocks.user.id = 99
    const w = render(); await open(w)
    expect(w.text()).toContain(L.creatorOnly)
    expect(w.find('[data-testid="review-confirm"]').exists()).toBe(false)
    expect(w.find('[data-testid="cancel-preview"]').exists()).toBe(false)
    mocks.user.id = 7
    mocks.api.get.mockResolvedValue(batch({ expires_at: '2026-10-25T01:00:00Z' }))
    await w.get('[data-testid="dialog-close"]').trigger('click'); await open(w)
    expect(w.find('[data-testid="review-confirm"]').exists()).toBe(false)
    expect(w.text()).toContain(L.conflictHint)
  })

  it('retries only a failed sync, without pretending the job completed', async () => {
    mocks.api.get.mockResolvedValue(batch({ status: 'failed', failed_phase: 'syncing', error_code: 'sync_failed', error_message: 'Cache synchronization failed; retry synchronization' }))
    mocks.api.retrySync.mockResolvedValue(batch({ status: 'syncing' }))
    const w = render(); await open(w)
    expect(w.text()).toContain(L.errors.sync_failed)
    expect(w.text()).not.toContain('Cache synchronization failed')
    expect(w.find('[data-testid="preview-again"]').exists()).toBe(false)
    mocks.api.get.mockResolvedValue(batch({ status: 'syncing' }))
    await w.get('[data-testid="retry-sync"]').trigger('click'); await flushPromises()
    expect(mocks.api.retrySync).toHaveBeenCalledTimes(1)
    expect(mocks.api.retrySync).toHaveBeenCalledWith(1)
    expect(mocks.api.confirm).not.toHaveBeenCalled()
    expect(w.emitted('changed')).toBeUndefined()
    expect(w.text()).toContain(L.syncWarning)
  })

  it('cancels an unconfirmed preview without touching confirmation', async () => {
    mocks.api.cancel.mockResolvedValue(batch({ status: 'cancelled' }))
    const w = render(); await open(w)
    mocks.api.get.mockResolvedValue(batch({ status: 'cancelled' }))
    await w.get('[data-testid="cancel-preview"]').trigger('click'); await flushPromises()
    expect(mocks.api.cancel).toHaveBeenCalledWith(1)
    expect(mocks.api.confirm).not.toHaveBeenCalled()
    expect(w.text()).toContain(L.cancelled)
    expect(w.find('[data-testid="cancel-preview"]').exists()).toBe(false)
    expect(w.find('[data-testid="review-confirm"]').exists()).toBe(false)
    expect(w.find('[data-testid="preview-again"]').exists()).toBe(true)
  })

  it('can cancel while the preview is still running', async () => {
    mocks.api.get.mockResolvedValue(batch({ status: 'previewing', scanned: 0, changed: 0, unchanged: 0, skipped: 0 }))
    mocks.api.cancel.mockResolvedValue(batch({ status: 'cancelled', scanned: 0, changed: 0, unchanged: 0, skipped: 0 }))
    const w = render(); await open(w)
    await w.get('[data-testid="cancel-preview"]').trigger('click'); await flushPromises()
    expect(mocks.api.cancel).toHaveBeenCalledWith(1)
  })

  it('translates server error codes instead of showing raw English messages', async () => {
    mocks.api.confirm.mockRejectedValue({ status: 409, reason: 'FINANCE_RECALCULATION_RULE_CHANGED', message: 'Cost configuration changed; create a new preview' })
    const w = render(); await open(w)
    await w.get('[data-testid="review-confirm"]').trigger('click')
    await w.get('[data-testid="acknowledge"]').setValue(true)
    await w.get('[data-testid="confirm-execute"]').trigger('click'); await flushPromises()
    expect(w.text()).toContain(L.errors.rule_changed)
    expect(w.text()).not.toContain('Cost configuration changed')
  })

  it('pre-fills a new preview from the previous scope', async () => {
    mocks.api.get.mockResolvedValue(batch({ status: 'expired', start: '2026-09-01', end: '2026-09-30', account_id: 11, model: 'historical-model', accept_retained_baseline: true }))
    const w = render(); await open(w)
    await w.get('[data-testid="preview-again"]').trigger('click'); await flushPromises()
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.api.create).toHaveBeenCalledWith(expect.objectContaining({ kind: 'usage_cost', start: '2026-09-01', end: '2026-09-30', account_id: 11, model: 'historical-model', accept_retained_baseline: true }))
  })

  it('does not offer sync retry for preview failures', async () => {
    mocks.api.get.mockResolvedValue(batch({ status: 'failed', failed_phase: 'previewing' }))
    const w = render(); await open(w)
    expect(w.find('[data-testid="retry-sync"]').exists()).toBe(false)
  })

  it('polls durable jobs, emits changed once on completion, and stops after unmount', async () => {
    mocks.api.list.mockResolvedValue({ items: [batch({ status: 'syncing' })], total: 1 })
    const w = render(); await flushPromises()
    expect(w.emitted('changed')).toBeUndefined()
    mocks.api.list.mockResolvedValue({ items: [batch({ status: 'completed' })], total: 1 })
    await vi.advanceTimersByTimeAsync(3000); await flushPromises()
    expect(w.emitted('changed')).toHaveLength(1)
    w.unmount()
    const calls = mocks.api.list.mock.calls.length
    await vi.advanceTimersByTimeAsync(10000)
    expect(mocks.api.list).toHaveBeenCalledTimes(calls)
  })

  it('ignores rejected detail requests for a previously selected batch', async () => {
    mocks.api.list.mockResolvedValue({ items: [batch(), batch({ id: 2 })], total: 2 })
    const old = deferred<RecalculationBatch>()
    mocks.api.get.mockReturnValueOnce(old.promise).mockResolvedValue(batch({ id: 2, model: 'new-model' }))
    const w = render(); await open(w)
    await w.get('[data-testid="dialog-close"]').trigger('click'); await open(w, 2)
    old.reject({ message: 'STALE FAILURE' }); await flushPromises()
    expect(w.text()).toContain('new-model')
    expect(w.text()).not.toContain('STALE FAILURE')
  })

  it('ignores a preview response after its dialog is closed', async () => {
    const response = deferred<RecalculationBatch>()
    mocks.api.create.mockReturnValue(response.promise)
    const w = render(); await newPreview(w)
    await w.get('[data-testid="dialog-close"]').trigger('click')
    response.resolve(batch()); await flushPromises()
    expect(w.find('[data-testid="review-confirm"]').exists()).toBe(false)
    expect(w.emitted('changed')).toBeUndefined()
  })

  it('ignores a stale rejected list response after refresh', async () => {
    const old = deferred<{ items: RecalculationBatch[]; total: number }>()
    mocks.api.list.mockReturnValueOnce(old.promise).mockResolvedValue({ items: [batch({ id: 2 })], total: 1 })
    const w = render()
    await w.setProps({ refreshToken: 1 }); await flushPromises()
    old.reject({ message: 'STALE LIST FAILURE' }); await flushPromises()
    expect(w.find('[data-testid="batch-2"]').exists()).toBe(true)
    expect(w.text()).not.toContain('STALE LIST FAILURE')
    expect(mocks.api.list.mock.calls[0][1].aborted).toBe(true)
  })

  it('uses the existing BaseDialog Escape behavior without cancelling the durable job', async () => {
    const w = render(true)
    await flushPromises()
    await w.get('[data-testid="batch-1"]').trigger('click'); await flushPromises()
    expect(document.body.querySelector('[role="dialog"]')).not.toBeNull()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(500)
    expect(document.body.querySelector('[role="dialog"]')).toBeNull()
    expect(mocks.api.confirm).not.toHaveBeenCalled()
    expect(mocks.api.retrySync).not.toHaveBeenCalled()
  })

  it('keeps standalone locale keys aligned', () => {
    function keys(value: object, prefix = ''): string[] {
      return Object.entries(value).flatMap(([key, child]) => typeof child === 'object' ? keys(child, `${prefix}${key}.`) : [`${prefix}${key}`]).sort()
    }
    expect(keys(zh)).toEqual(keys(en))
  })
})
