/**
 * CUSTOM: 美元汇率（管理员）。汇率只用于显示「相比官方价」折扣，不参与计费。
 * 接口约定见任务 design.md §15.4。
 */

import { apiClient } from '../client'

export interface FxRateView {
  auto_enabled: boolean
  default_rate: number
  effective_rate: number
  effective_from: 'fetched' | 'default'
  fetched_rate: number | null
  /** RFC3339 */
  fetched_at: string | null
  /** 最近一次成功的来源名（URL 的 host），没有为 ""。 */
  source: string
  /** 开启自动更新且（从未成功或 fetched_at 超过 3 天）。 */
  stale: boolean
  /** 本进程最近一次失败原因，没有为 ""。 */
  last_error: string
  last_attempt_at: string | null
}

export interface UpdateFxRateRequest {
  auto_enabled: boolean
  /** 5–9 */
  default_rate: number
}

export interface FxRateRefreshResult {
  ok: boolean
  error: string
  rate: FxRateView
}

export async function get(): Promise<FxRateView> {
  const { data } = await apiClient.get<FxRateView>('/admin/settings/fx-rate')
  return data
}

export async function update(body: UpdateFxRateRequest): Promise<FxRateView> {
  const { data } = await apiClient.put<FxRateView>('/admin/settings/fx-rate', body)
  return data
}

/** 立即更新；全部来源失败时 ok=false、HTTP 仍为 200。 */
export async function refresh(): Promise<FxRateRefreshResult> {
  const { data } = await apiClient.post<FxRateRefreshResult>('/admin/settings/fx-rate/refresh')
  return data
}

export const fxRateAPI = { get, update, refresh }

export default fxRateAPI
