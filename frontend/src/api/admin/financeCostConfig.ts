/** Cost-only conditional channel writes. Never fall back to the unguarded channel update API. */
import { apiClient } from '../client'
import type { AccountStatsPricingRule } from './channels'
export interface FinanceChannelCostConfig {
  channel_id: number
  version: string
  apply_pricing_to_account_stats: boolean
  account_stats_pricing_rules: AccountStatsPricingRule[]
}
export interface FinanceChannelCostUpdate {
  expected_version: string
  apply_pricing_to_account_stats?: boolean
  account_stats_pricing_rules?: AccountStatsPricingRule[]
}
export async function getChannelCostConfig(id: number): Promise<FinanceChannelCostConfig> {
  const { data } = await apiClient.get<FinanceChannelCostConfig>(`/admin/finance/channel-costs/${id}`)
  return data
}
export async function updateChannelCostConfig(id: number, payload: FinanceChannelCostUpdate): Promise<FinanceChannelCostConfig> {
  const { data } = await apiClient.put<FinanceChannelCostConfig>(`/admin/finance/channel-costs/${id}`, payload)
  return data
}
