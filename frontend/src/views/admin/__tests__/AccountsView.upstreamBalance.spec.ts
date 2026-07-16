import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  queryUpstreamBalance,
  getBatchTodayStats,
  getAllProxies,
  getAllGroups,
  showSuccess,
  showWarning
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  queryUpstreamBalance: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      queryUpstreamBalance,
      getBatchTodayStats,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: { getAll: getAllProxies },
    groups: { getAll: getAllGroups }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess,
    showInfo: vi.fn(),
    showWarning
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token' })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'en-US' },
      t: (key: string, values?: Record<string, number>) => {
        if (!values) return key
        return `${key}:${values.success}:${values.failed}:${values.unsupported}`
      }
    })
  }
})

const DataTableStub = {
  props: ['data'],
  template: `
    <div>
      <div v-for="row in data" :key="row.id" :data-test="'account-' + row.id">
        <slot name="cell-upstream_balance" :row="row" />
      </div>
    </div>
  `
}

const commonStubs = {
  AppLayout: { template: '<div><slot /></div>' },
  TablePageLayout: {
    template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
  },
  DataTable: DataTableStub,
  Pagination: true,
  ConfirmDialog: true,
  AccountTableActions: { template: '<div><slot name="after" /></div>' },
  AccountTableFilters: true,
  AccountBulkActionsBar: true,
  AccountActionMenu: true,
  ImportDataModal: true,
  ReAuthAccountModal: true,
  AccountTestModal: true,
  AccountStatsModal: true,
  ScheduledTestsPanel: true,
  SyncFromCrsModal: true,
  TempUnschedStatusModal: true,
  ErrorPassthroughRulesModal: true,
  TLSFingerprintProfilesModal: true,
  CreateAccountModal: true,
  EditAccountModal: true,
  BulkEditAccountModal: true,
  PlatformTypeBadge: true,
  AccountCapacityCell: true,
  AccountStatusIndicator: true,
  AccountTodayStatsCell: true,
  AccountGroupsCell: true,
  AccountUsageCell: true,
  Icon: true
}

function buildAccount(id: number, type: 'upstream' | 'oauth') {
  return {
    id,
    name: `account-${id}`,
    platform: 'openai',
    type,
    status: id % 2 === 0 ? 'inactive' : 'active',
    schedulable: true,
    created_at: '2026-07-16T10:00:00Z',
    updated_at: '2026-07-16T10:00:00Z'
  }
}

function mountAccountsView() {
  return mount(AccountsView, {
    global: { stubs: commonStubs }
  })
}

function getCurrentPageBalanceRefreshButton(wrapper: ReturnType<typeof mountAccountsView>) {
  return wrapper.get(
    'button[aria-label="admin.accounts.upstreamBalance.refreshCurrentPage"]'
  )
}

describe('AccountsView upstream balances', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()

    listAccounts.mockResolvedValue({
      items: [
        buildAccount(1, 'upstream'),
        buildAccount(2, 'upstream'),
        buildAccount(3, 'upstream'),
        buildAccount(4, 'upstream'),
        buildAccount(5, 'upstream'),
        buildAccount(6, 'oauth')
      ],
      total: 6,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listWithEtag.mockResolvedValue({ notModified: true, etag: null, data: null })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('refreshes only current-page upstream accounts with a global concurrency limit of four', async () => {
    let activeRequestCount = 0
    let maximumActiveRequestCount = 0
    const pendingRequestResolvers: Array<() => void> = []

    queryUpstreamBalance.mockImplementation(async (accountId: number) => {
      activeRequestCount += 1
      maximumActiveRequestCount = Math.max(maximumActiveRequestCount, activeRequestCount)
      await new Promise<void>((resolve) => {
        pendingRequestResolvers.push(resolve)
      })
      activeRequestCount -= 1
      return {
        status: 'available',
        platform_type: 'sub2api',
        scope: 'user',
        remaining: String(accountId),
        unit: 'USD',
        queried_at: '2026-07-16T12:00:00Z'
      }
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    const batchButton = getCurrentPageBalanceRefreshButton(wrapper)
    expect(batchButton.text()).toContain('admin.accounts.upstreamBalance.refresh')
    await batchButton.trigger('click')
    await flushPromises()

    expect(batchButton.attributes('aria-label')).toBe(
      'admin.accounts.upstreamBalance.refreshingCurrentPage'
    )
    expect(batchButton.attributes('aria-busy')).toBe('true')
    expect(queryUpstreamBalance).toHaveBeenCalledTimes(4)
    expect(maximumActiveRequestCount).toBe(4)

    pendingRequestResolvers.shift()?.()
    await flushPromises()
    expect(queryUpstreamBalance).toHaveBeenCalledTimes(5)
    expect(maximumActiveRequestCount).toBe(4)

    while (pendingRequestResolvers.length > 0) {
      pendingRequestResolvers.shift()?.()
      await flushPromises()
    }

    expect(queryUpstreamBalance).not.toHaveBeenCalledWith(6)
    expect(showSuccess).toHaveBeenCalledWith(
      'admin.accounts.upstreamBalance.batchSummary:5:0:0'
    )
    wrapper.unmount()
  })

  it('deduplicates a row refresh that overlaps a page refresh', async () => {
    let resolveAccountOne!: () => void
    queryUpstreamBalance.mockImplementation(async (accountId: number) => {
      if (accountId === 1) {
        await new Promise<void>((resolve) => {
          resolveAccountOne = resolve
        })
      }
      return {
        status: 'available',
        platform_type: 'sub2api',
        scope: 'user',
        remaining: '1',
        unit: 'USD',
        queried_at: '2026-07-16T12:00:00Z'
      }
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    await wrapper.get('[data-test="account-1"] button').trigger('click')
    await getCurrentPageBalanceRefreshButton(wrapper).trigger('click')
    await flushPromises()

    const accountOneCalls = queryUpstreamBalance.mock.calls.filter(([accountId]) => accountId === 1)
    expect(accountOneCalls).toHaveLength(1)

    resolveAccountOne()
    await flushPromises()
    wrapper.unmount()
  })

  it('continues after partial failures and reports all outcome categories', async () => {
    queryUpstreamBalance.mockImplementation(async (accountId: number) => {
      if (accountId === 2) {
        throw new Error('credential rejected')
      }
      if (accountId === 3) {
        return {
          status: 'unsupported',
          platform_type: 'unknown',
          scope: 'unknown',
          queried_at: '2026-07-16T12:00:00Z'
        }
      }
      return {
        status: 'available',
        platform_type: 'sub2api',
        scope: 'user',
        remaining: String(accountId),
        unit: 'USD',
        queried_at: '2026-07-16T12:00:00Z'
      }
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    await getCurrentPageBalanceRefreshButton(wrapper).trigger('click')
    await flushPromises()

    expect(queryUpstreamBalance).toHaveBeenCalledTimes(5)
    expect(queryUpstreamBalance).toHaveBeenCalledWith(2)
    expect(queryUpstreamBalance).not.toHaveBeenCalledWith(6)
    expect(showWarning).toHaveBeenCalledWith(
      'admin.accounts.upstreamBalance.batchSummary:3:1:1'
    )
    expect(wrapper.get('[data-test="account-1"]').text()).toContain('$1.00')
    expect(wrapper.get('[data-test="account-2"]').text()).toContain('credential rejected')
    expect(wrapper.get('[data-test="account-3"]').text()).toContain('admin.accounts.upstreamBalance.unsupported')
    wrapper.unmount()
  })
})
