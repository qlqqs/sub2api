import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import AdminFinanceView from '../AdminFinanceView.vue'
import FinanceRecalculationTab from '@/components/admin/finance/FinanceRecalculationTab.vue'
import { normalizeSummary } from '@/api/admin/finance'

const api = vi.hoisted(() => ({ getSummary: vi.fn(), exportCsv: vi.fn() }))
const route = vi.hoisted(() => ({ query: {} as Record<string, unknown> }))
const replace = vi.hoisted(() => vi.fn())
const confirmLeave = vi.hoisted(() => vi.fn().mockResolvedValue(true))

vi.mock('@/api/admin', () => ({ adminAPI: { finance: api } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ replace }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) })
  }
})

const DateRangePickerStub = {
  props: ['startDate', 'endDate'],
  emits: ['update:startDate', 'update:endDate'],
  template:
    '<div><input data-testid="start" :value="startDate" @input="$emit(\'update:startDate\', $event.target.value)" /><input data-testid="end" :value="endDate" @input="$emit(\'update:endDate\', $event.target.value)" /></div>'
}

function mountView() {
  return mount(AdminFinanceView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        DateRangePicker: DateRangePickerStub,
        Icon: true,
        FinanceOverviewTab: { props: ['summary'], template: '<div data-testid="overview-stub" />' },
        FinanceProfitTable: { props: ['summary'], template: '<div data-testid="profit-stub" />' },
        FinanceRechargeTab: { props: ['start', 'end', 'refreshToken'], template: '<div data-testid="recharge-stub">{{ start }}~{{ end }}</div>' },
        FinanceBreakdownTab: true,
        FinanceAffiliateTab: true,
        FinanceExpensesTab: true,
        FinanceReconcileTab: true,
        FinanceSettingsTab: {
          props: ['refreshToken'], emits: ['changed'],
          setup(_, { expose }) { expose({ confirmLeave }); return {} },
          template: '<div data-testid="settings-stub"><slot name="history" :refresh-token="refreshToken" :changed="() => $emit(\'changed\')" /></div>'
        },
        FinanceRecalculationTab: { props: ['refreshToken'], emits: ['changed'], template: '<div data-testid="history-stub" />' }
      }
    }
  })
}

describe('AdminFinanceView (CUSTOM)', () => {
  beforeEach(() => {
    route.query = reactive({})
    replace.mockReset()
    confirmLeave.mockReset().mockResolvedValue(true)
    api.getSummary.mockReset().mockResolvedValue(
      normalizeSummary({ warnings: ['范围内有 2 笔订阅订单，未计入利润'], computed_at: '2026-10-25T21:00:00+08:00' })
    )
  })

  it('loads the summary for the default range and shows warnings on top', async () => {
    const wrapper = mountView()
    await flushPromises()
    const [query] = api.getSummary.mock.calls[0]
    expect(query.granularity).toBe('day')
    expect(query.start).toMatch(/^\d{4}-\d{2}-01$/)
    expect(wrapper.get('[data-testid="finance-warnings"]').text()).toContain('范围内有 2 笔订阅订单，未计入利润')
    expect(wrapper.find('[data-testid="overview-stub"]').exists()).toBe(true)
  })

  it('hides the warning banner when there are no warnings', async () => {
    api.getSummary.mockResolvedValue(normalizeSummary({ warnings: [] }))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="finance-warnings"]').exists()).toBe(false)
  })

  it('rejects an inverted range without refetching', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="start"]').setValue('2026-10-20')
    await flushPromises()
    const calls = api.getSummary.mock.calls.length
    await wrapper.get('[data-testid="end"]').setValue('2026-10-10')
    await flushPromises()
    expect(wrapper.get('[data-testid="finance-range-error"]').text()).toBe('admin.finance.toolbar.rangeInvalid')
    expect(api.getSummary).toHaveBeenCalledTimes(calls)
    expect(wrapper.get('[data-testid="finance-export"]').attributes('disabled')).toBeDefined()
  })

  it('refetches with the month granularity and passes the range to lazy tabs', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="finance-granularity-month"]').trigger('click')
    await flushPromises()
    expect(api.getSummary.mock.calls.at(-1)?.[0].granularity).toBe('month')

    await wrapper.get('[data-testid="finance-tab-recharge"]').trigger('click')
    expect(replace).toHaveBeenCalledWith({ query: { tab: 'recharge' } })
    expect(wrapper.get('[data-testid="recharge-stub"]').text()).toMatch(/^\d{4}-\d{2}-01~\d{4}-\d{2}-\d{2}$/)
  })
  it('hides report toolbar and report warnings on settings', async () => {
    route.query = reactive({ tab: 'settings' })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="finance-report-toolbar"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="finance-export"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="finance-warnings"]').exists()).toBe(false)
    await wrapper.get('[data-testid="finance-tab-overview"]').trigger('click'); await flushPromises()
    expect(wrapper.find('[data-testid="finance-report-toolbar"]').exists()).toBe(true)
  })

  it('does not unmount settings or update the URL until unsaved changes are confirmed', async () => {
    route.query = reactive({ tab: 'settings' })
    confirmLeave.mockResolvedValueOnce(false)
    const wrapper = mountView(); await flushPromises()
    await wrapper.get('[data-testid="finance-tab-overview"]').trigger('click'); await flushPromises()
    expect(confirmLeave).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-testid="settings-stub"]').exists()).toBe(true)
    expect(replace).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="finance-tab-overview"]').trigger('click'); await flushPromises()
    expect(wrapper.find('[data-testid="settings-stub"]').exists()).toBe(false)
    expect(replace).toHaveBeenCalledWith({ query: { tab: undefined } })
  })

  it('mounts historical recalculation in the settings history slot and reloads the summary on completion', async () => {
    route.query = reactive({ tab: 'settings', section: 'history' })
    const wrapper = mountView(); await flushPromises()
    const history = wrapper.getComponent(FinanceRecalculationTab)
    expect(history.props('refreshToken')).toBe(0)
    const before = api.getSummary.mock.calls.length
    history.vm.$emit('changed'); await flushPromises()
    expect(api.getSummary).toHaveBeenCalledTimes(before + 1)
  })

})
