import { beforeEach, expect, it, vi } from 'vitest'
import { getChannelCostConfig, updateChannelCostConfig } from '../financeCostConfig'
const client = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))
beforeEach(() => vi.clearAllMocks())
it('reads the versioned cost-only resource and sends expected_version without unrelated fields', async () => {
  const config = { channel_id: 8, version: 'sha256-current', apply_pricing_to_account_stats: false, account_stats_pricing_rules: [] }
  client.get.mockResolvedValue({ data: config })
  client.put.mockResolvedValue({ data: { ...config, version: 'sha256-next' } })
  expect(await getChannelCostConfig(8)).toEqual(config)
  expect(client.get).toHaveBeenCalledWith('/admin/finance/channel-costs/8')
  await updateChannelCostConfig(8, { expected_version: config.version, account_stats_pricing_rules: [] })
  expect(client.put).toHaveBeenCalledWith('/admin/finance/channel-costs/8', { expected_version: 'sha256-current', account_stats_pricing_rules: [] })
})
it('propagates conflicts and never retries through an unconditional endpoint', async () => {
  const error = { status: 409, reason: 'FINANCE_COST_CONFIG_CHANGED' }
  client.put.mockRejectedValue(error)
  await expect(updateChannelCostConfig(8, { expected_version: 'old' })).rejects.toEqual(error)
  expect(client.put).toHaveBeenCalledOnce()
})
