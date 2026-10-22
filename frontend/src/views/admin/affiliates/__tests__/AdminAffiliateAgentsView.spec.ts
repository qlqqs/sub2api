import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminAffiliateAgentsView from '../AdminAffiliateAgentsView.vue'

const {
  listUsers,
  lookupUsers,
  updateUserSettings,
  clearUserSettings,
  batchSetAgentLevel,
  listUserCustomers,
  routerPush,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  lookupUsers: vi.fn(),
  updateUserSettings: vi.fn(),
  clearUserSettings: vi.fn(),
  batchSetAgentLevel: vi.fn(),
  listUserCustomers: vi.fn(),
  routerPush: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin/affiliates', () => {
  const affiliatesAPI = {
    listUsers,
    lookupUsers,
    updateUserSettings,
    clearUserSettings,
    batchSetAgentLevel,
    listUserCustomers,
  }
  return { affiliatesAPI, default: affiliatesAPI }
})

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}${JSON.stringify(params)}` : key,
    }),
  }
})

const TablePageLayoutStub = {
  template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
}

const DataTableStub = {
  props: ['columns', 'data', 'selectedKeys'],
  emits: ['update:selectedKeys'],
  template: `
    <table>
      <tr v-for="(row, index) in data" :key="index" :data-test="'row-' + index">
        <td v-for="column in columns" :key="column.key" :data-test="'cell-' + column.key">
          <slot :name="'cell-' + column.key" :row="row" />
        </td>
      </tr>
    </table>
  `,
}

const BaseDialogStub = {
  props: ['show', 'title'],
  emits: ['close'],
  template: '<div v-if="show" data-test="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></div>',
}

const ConfirmDialogStub = {
  props: ['show', 'title', 'message'],
  emits: ['confirm', 'cancel'],
  template: `<div v-if="show" data-test="confirm"><p data-test="confirm-message">{{ message }}</p><button type="button" data-test="confirm-ok" @click="$emit('confirm')">ok</button></div>`,
}

function entry(overrides: Record<string, unknown> = {}) {
  return {
    user_id: 2,
    email: 'b@x.com',
    username: 'b',
    aff_code: 'VIP',
    aff_code_custom: true,
    aff_rebate_rate_percent: 25,
    aff_count: 8,
    agent_level: 2,
    agent_level_locked: true,
    agent_level_source: 'agent',
    agent_level_updated_by: 1,
    agent_level_updated_by_email: 'a@x.com',
    agent_owner_id: 1,
    agent_owner_email: 'a@x.com',
    agent_owner_level: 1,
    agent_owner_deleted: false,
    agent_upline_id: null,
    agent_upline_email: '',
    agent_upline_deleted: false,
    owned_customer_count: 15,
    agent_profit_total: 96,
    ...overrides,
  }
}

function page<T>(items: T[]) {
  return { items, total: items.length, page: 1, page_size: 20, pages: 1 }
}

function mountView() {
  return mount(AdminAffiliateAgentsView, {
    attachTo: document.body,
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: true,
        Icon: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: ConfirmDialogStub,
        Teleport: true,
      },
    },
  })
}

describe('AdminAffiliateAgentsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers()
  })

  it('renders agents with level source, ownership, counters and exclusive rate', async () => {
    listUsers.mockResolvedValue(page([
      entry(),
      entry({
        user_id: 3,
        email: 'c@x.com',
        aff_code_custom: false,
        aff_rebate_rate_percent: null,
        aff_count: 2,
        agent_level: 3,
        agent_level_locked: false,
        agent_level_source: 'manual',
        agent_level_updated_by_email: '',
        agent_owner_id: 9,
        agent_owner_email: '',
        agent_owner_level: 2,
        agent_owner_deleted: true,
        owned_customer_count: 0,
        agent_profit_total: 0,
      }),
    ]))

    const wrapper = mountView()
    await flushPromises()

    expect(listUsers).toHaveBeenCalledWith(expect.objectContaining({ page: 1, search: '', agent_level: null, owner_id: null }))

    const first = wrapper.get('[data-test="row-0"]')
    expect(first.get('[data-test="cell-agent_level"]').text()).toContain('admin.affiliates.levels.2')
    expect(first.get('[data-test="agent-level-source"]').text()).toBe('admin.affiliates.agents.source.agent{"email":"a@x.com"}')
    expect(first.get('[data-test="cell-owner"]').text()).toContain('a@x.com')
    expect(first.get('[data-test="cell-aff_count"]').text()).toBe('8')
    expect(first.get('[data-test="cell-owned_customer_count"]').text()).toBe('15')
    expect(first.get('[data-test="cell-agent_profit_total"]').text()).toBe('$96.00')
    expect(first.get('[data-test="cell-custom_rate"]').text()).toBe('25%')

    const second = wrapper.get('[data-test="row-1"]')
    expect(second.get('[data-test="agent-level-source"]').text()).toBe('admin.affiliates.agents.source.manual')
    expect(second.get('[data-test="cell-owner"]').text()).toBe('admin.affiliates.deleted')
    expect(second.get('[data-test="cell-owned_customer_count"]').text()).toBe('—')
    expect(second.get('[data-test="cell-custom_rate"]').text()).toBe('—')
    wrapper.unmount()
  })

  it('filters by level and asks for confirmation before demoting', async () => {
    listUsers.mockResolvedValue(page([entry()]))
    updateUserSettings.mockResolvedValue({ user_id: 2 })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="agents-level-filter"]').setValue('2')
    await flushPromises()
    expect(listUsers).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, agent_level: 2 }))

    await wrapper.get('[data-test="agent-adjust"]').trigger('click')
    await wrapper.get('[data-test="agent-adjust-level"]').setValue('3')
    await wrapper.get('[data-test="agent-adjust-submit"]').trigger('click')

    expect(updateUserSettings).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="confirm-message"]').text()).toBe('admin.affiliates.agents.demoteMessage')

    await wrapper.get('[data-test="confirm-ok"]').trigger('click')
    await flushPromises()
    expect(updateUserSettings).toHaveBeenCalledWith(2, { agent_level: 3 })
    wrapper.unmount()
  })

  it('saves promotions and rate changes without a confirmation', async () => {
    listUsers.mockResolvedValue(page([entry()]))
    updateUserSettings.mockResolvedValue({ user_id: 2 })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="agent-adjust"]').trigger('click')
    expect(wrapper.get('[data-test="agent-adjust-submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="agent-adjust-level"]').setValue('1')
    await wrapper.get('[data-test="agent-adjust-rate"]').setValue('')
    await wrapper.get('[data-test="agent-adjust-unlock"]').setValue(true)
    await wrapper.get('[data-test="agent-adjust-submit"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="confirm"]').exists()).toBe(false)
    expect(updateUserSettings).toHaveBeenCalledWith(2, {
      agent_level: 1,
      clear_rebate_rate: true,
      agent_level_locked: false,
    })
    wrapper.unmount()
  })

  it('opens customers and rebate records from the more menu', async () => {
    listUsers.mockResolvedValue(page([entry()]))
    listUserCustomers.mockResolvedValue(page([
      { user_id: 7, email: 'c***@x.com', username: 'c', agent_level: 3, relation: 'upline', bound_at: '2026-10-01T00:00:00Z', contribution: 3 },
    ]))

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="agent-more"]').trigger('click')
    await wrapper.get('[data-test="agent-menu-customers"]').trigger('click')
    await flushPromises()
    expect(listUserCustomers).toHaveBeenCalledWith(2, { page: 1, page_size: 20 })
    const table = wrapper.get('[data-test="agent-customers-table"]')
    expect(table.text()).toContain('admin.affiliates.agents.customers.relationUpline')
    expect(table.text()).toContain('$3.00')

    await wrapper.get('[data-test="agent-more"]').trigger('click')
    await wrapper.get('[data-test="agent-menu-rebates"]').trigger('click')
    expect(routerPush).toHaveBeenCalledWith({ path: '/admin/affiliates/rebates', query: { search: 'b@x.com' } })
    wrapper.unmount()
  })

  it('sets a new agent from a user lookup', async () => {
    vi.useFakeTimers()
    listUsers.mockResolvedValue(page([]))
    lookupUsers.mockResolvedValue([{ id: 42, email: 'new@x.com', username: 'new' }])
    updateUserSettings.mockResolvedValue({ user_id: 42 })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="agents-add-open"]').trigger('click')
    expect(wrapper.get('[data-test="agent-add-submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="agent-add-user-input"]').setValue('new')
    vi.advanceTimersByTime(300)
    await flushPromises()
    await wrapper.get('[data-test="agent-add-user-option"]').trigger('click')
    await wrapper.get('[data-test="agent-add-level"]').setValue('1')
    await wrapper.get('[data-test="agent-add-submit"]').trigger('click')
    await flushPromises()

    expect(updateUserSettings).toHaveBeenCalledWith(42, { agent_level: 1 })
    expect(listUsers).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    vi.useRealTimers()
  })
})
