/** Durable historical repairs. Decimal strings and NULLs are never coerced to zero. */
import { apiClient } from '../client'

export type RecalculationKind = 'usage_cost' | 'payment_fee'
export type RecalculationStatus = 'previewing' | 'ready' | 'queued' | 'applying' | 'syncing' | 'completed' | 'failed' | 'conflict' | 'expired' | 'cancelled'
export interface RecalculationScope {
  kind: RecalculationKind
  start: string
  end: string
  account_id?: number | null
  provider_id?: number | null
  model?: string
  accept_retained_baseline?: boolean
}
export interface RecalculationBatch extends RecalculationScope {
  id: number
  status: RecalculationStatus
  scanned: number
  changed: number
  unchanged: number
  skipped: number
  old_total: string
  new_total: string
  delta: string
  actor_id: number
  config_fingerprint: string
  cutoff_at: string
  expires_at: string
  created_at: string
  error_code: string
  error_message: string
  /** Public, sanitized source summary only; never render a raw configuration object. */
  amount_unit?: 'account_cost' | 'CNY'
  snapshot_summary?: string[]
  failed_phase?: string
}
export interface RecalculationItem {
  id: number
  status: string
  reason: string
  source: string
  old_cost: string | null
  new_cost: string | null
  delta: string | null
  applied: boolean
}
export interface RecalculationPage<T> { items: T[]; total: number }
export interface RecalculationPageQuery { page: number; page_size: number }
const root = '/admin/finance/recalculations'

export const financeRecalculationAPI = {
  async create(payload: RecalculationScope & { idempotency_key: string }): Promise<RecalculationBatch> {
    // Explicit request whitelist: a hidden usage acknowledgement must never leak into fees.
    const request: RecalculationScope & { idempotency_key: string } = {
      kind: payload.kind, start: payload.start, end: payload.end, idempotency_key: payload.idempotency_key
    }
    if (payload.kind === 'usage_cost') {
      if (payload.account_id !== undefined) request.account_id = payload.account_id
      if (payload.model !== undefined) request.model = payload.model
      if (payload.accept_retained_baseline !== undefined) request.accept_retained_baseline = payload.accept_retained_baseline === true
    } else if (payload.provider_id !== undefined) request.provider_id = payload.provider_id
    const { data } = await apiClient.post<RecalculationBatch>(root, request)
    return data
  },
  async list(params: RecalculationPageQuery, signal?: AbortSignal): Promise<RecalculationPage<RecalculationBatch>> {
    const { data } = await apiClient.get<RecalculationPage<RecalculationBatch>>(root, { params, signal })
    return data
  },
  async get(id: number, signal?: AbortSignal): Promise<RecalculationBatch> {
    const { data } = await apiClient.get<RecalculationBatch>(`${root}/${id}`, { signal })
    return data
  },
  async items(id: number, params: RecalculationPageQuery, signal?: AbortSignal): Promise<RecalculationPage<RecalculationItem>> {
    const { data } = await apiClient.get<RecalculationPage<RecalculationItem>>(`${root}/${id}/items`, { params, signal })
    return data
  },
  async confirm(id: number, config_fingerprint: string): Promise<RecalculationBatch> {
    const { data } = await apiClient.post<RecalculationBatch>(`${root}/${id}/confirm`, { config_fingerprint, acknowledged: true })
    return data
  },
  /** Discards an unconfirmed preview; confirmed batches cannot be cancelled. */
  async cancel(id: number): Promise<RecalculationBatch> {
    const { data } = await apiClient.post<RecalculationBatch>(`${root}/${id}/cancel`)
    return data
  },
  async retrySync(id: number): Promise<RecalculationBatch> {
    const { data } = await apiClient.post<RecalculationBatch>(`${root}/${id}/retry-sync`)
    return data
  }
}

export function recalculationIsRunning(status: RecalculationStatus): boolean {
  return ['previewing', 'queued', 'applying', 'syncing'].includes(status)
}
