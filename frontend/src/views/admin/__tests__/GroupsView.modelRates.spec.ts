import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminGroup } from '@/types'
import GroupsView from '@/views/admin/GroupsView.vue'

// CUSTOM: 分组内单个模型倍率在分组弹窗中的保存时机、失败提示与列表倍率列提示。

const api = vi.hoisted(() => ({
  listGroups: vi.fn(),
  createGroup: vi.fn(),
  updateGroup: vi.fn(),
  getModelAllowlistCandidates: vi.fn(),
  getUsageSummary: vi.fn(),
  getCapacitySummary: vi.fn(),
  getLiveCapability: vi.fn(),
  listAllModelRates: vi.fn(),
  getModelRates: vi.fn(),
  replaceModelRates: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

const authState = vi.hoisted(() => ({ isSimpleMode: false }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list: api.listGroups,
      duplicate: vi.fn(),
      getModelAllowlistCandidates: api.getModelAllowlistCandidates,
      getUsageSummary: api.getUsageSummary,
      getCapacitySummary: api.getCapacitySummary,
      getLiveCapability: api.getLiveCapability,
      getAll: vi.fn().mockResolvedValue([]),
      create: api.createGroup,
      update: api.updateGroup,
      delete: vi.fn(),
      updateSortOrder: vi.fn()
    },
    accounts: { list: vi.fn(), getById: vi.fn() },
    groupModelRates: {
      listAll: api.listAllModelRates,
      getForGroup: api.getModelRates,
      replaceForGroup: api.replaceModelRates
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: api.showSuccess, showError: api.showError })
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }))
vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({ isCurrentStep: vi.fn(() => false), nextStep: vi.fn() })
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const group = {
  id: 42,
  name: 'Primary',
  description: null,
  platform: 'openai',
  rate_multiplier: 1,
  rpm_limit: 0,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  allow_image_generation: false,
  allow_batch_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  batch_image_discount_multiplier: 0.5,
  batch_image_hold_multiplier: 0.6,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  video_price_480p: null,
  video_price_720p: null,
  video_price_1080p: null,
  web_search_price_per_call: null,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  default_mapped_model: '',
  messages_dispatch_model_config: undefined,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: '2026-07-16T00:00:00Z',
  updated_at: '2026-07-16T00:00:00Z',
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  account_count: 1,
  active_account_count: 1,
  rate_limited_account_count: 0,
  model_allowlist: undefined,
  sort_order: 10
} as unknown as AdminGroup

const DataTableStub = defineComponent({
  props: {
    data: { type: Array, default: () => [] },
    columns: { type: Array, default: () => [] },
    loading: { type: Boolean, default: false }
  },
  template: `<div>
    <div v-for="row in data" :key="row.id">
      <div data-testid="rate-cell"><slot name="cell-rate_multiplier" :row="row" :value="row.rate_multiplier" /></div>
      <slot name="cell-actions" :row="row" />
    </div>
  </div>`
})

const BaseDialogStub = defineComponent({
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

function mountView() {
  return mount(GroupsView, {
    global: {
      stubs: {
        AppLayout: defineComponent({ template: '<main><slot /></main>' }),
        TablePageLayout: defineComponent({
          template: '<section><slot name="filters" /><slot name="table" /><slot name="pagination" /></section>'
        }),
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        Select: true,
        PlatformIcon: true,
        Icon: true,
        GroupCapacityBadge: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        VueDraggable: true
      }
    }
  })
}

async function openEdit(wrapper: ReturnType<typeof mountView>) {
  const editButton = wrapper.findAll('button').find((button) => button.text() === 'common.edit')
  expect(editButton).toBeTruthy()
  await editButton!.trigger('click')
  await flushPromises()
}

async function addRule(wrapper: ReturnType<typeof mountView>, model: string, rate: string) {
  await wrapper.get('[data-testid="group-model-rates-add"]').trigger('click')
  const models = wrapper.findAll('[data-testid="group-model-rates-model"]')
  const rates = wrapper.findAll('[data-testid="group-model-rates-rate"]')
  await models[models.length - 1].setValue(model)
  await rates[rates.length - 1].setValue(rate)
}

describe('GroupsView per-model rates (CUSTOM)', () => {
  beforeEach(() => {
    authState.isSimpleMode = false
    localStorage.clear()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    for (const fn of Object.values(api)) fn.mockReset()
    api.listGroups.mockResolvedValue({ items: [group], total: 1, page: 1, page_size: 20, pages: 1 })
    api.getModelAllowlistCandidates.mockResolvedValue([])
    api.getUsageSummary.mockResolvedValue([])
    api.getCapacitySummary.mockResolvedValue([])
    api.getLiveCapability.mockResolvedValue({ supported: false })
    api.listAllModelRates.mockResolvedValue({})
    api.getModelRates.mockResolvedValue([])
    api.updateGroup.mockResolvedValue(group)
    api.createGroup.mockResolvedValue({ ...group, id: 77 })
    api.replaceModelRates.mockImplementation(async (_id: number, rules: unknown[]) => rules)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows "N per-model rates" in the rate column with the rules on hover', async () => {
    api.listAllModelRates.mockResolvedValue({
      42: [
        { model: 'claude-opus-*', rate_multiplier: 1.5 },
        { model: 'gpt-5.5', rate_multiplier: 0.8 }
      ]
    })
    const wrapper = mountView()
    await flushPromises()
    const summary = wrapper.get('[data-testid="group-model-rates-summary"]')
    expect(summary.text()).toBe('admin.groups.modelRates.count')
    expect(summary.attributes('title')).toBe('claude-opus-*: 1.5x\ngpt-5.5: 0.8x')
    wrapper.unmount()
  })

  it('omits the hint for groups without rules', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="rate-cell"]').text()).toBe('1x')
    wrapper.unmount()
  })

  it('saves per-model rates after the group update succeeds', async () => {
    api.getModelRates.mockResolvedValue([{ model: 'claude-opus-*', rate_multiplier: 1.5 }])
    const wrapper = mountView()
    await flushPromises()
    await openEdit(wrapper)
    expect(api.getModelRates).toHaveBeenCalledWith(42)

    await addRule(wrapper, 'gpt-5.5', '0.8')
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(api.updateGroup).toHaveBeenCalledTimes(1)
    expect(api.replaceModelRates).toHaveBeenCalledWith(42, [
      { model: 'claude-opus-*', rate_multiplier: 1.5 },
      { model: 'gpt-5.5', rate_multiplier: 0.8 }
    ])
    expect(api.updateGroup.mock.invocationCallOrder[0]).toBeLessThan(api.replaceModelRates.mock.invocationCallOrder[0])
    expect(api.showSuccess).toHaveBeenCalledWith('admin.groups.groupUpdated')
    expect(api.showError).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('does not call the model-rate API when nothing changed', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openEdit(wrapper)
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(api.updateGroup).toHaveBeenCalledTimes(1)
    expect(api.replaceModelRates).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('reports "group saved, per-model rates failed" without undoing the group update', async () => {
    api.replaceModelRates.mockRejectedValueOnce({ message: 'invalid pattern' })
    const wrapper = mountView()
    await flushPromises()
    await openEdit(wrapper)
    await addRule(wrapper, 'gpt-5.5', '0.8')
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(api.showSuccess).toHaveBeenCalledWith('admin.groups.groupUpdated')
    expect(api.showError).toHaveBeenCalledWith('admin.groups.modelRates.saveFailed')
    wrapper.unmount()
  })

  it('blocks submitting the group while a per-model rate is invalid', async () => {
    const wrapper = mountView()
    await flushPromises()
    await openEdit(wrapper)
    await addRule(wrapper, 'claude-*-opus', '1')
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(api.updateGroup).not.toHaveBeenCalled()
    expect(api.showError).toHaveBeenCalledWith('admin.groups.modelRates.errors.wildcardPosition')
    wrapper.unmount()
  })

  it('saves per-model rates for a newly created group', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
    await flushPromises()
    expect(api.getModelRates).not.toHaveBeenCalled()
    await wrapper.get('#create-group-form [data-tour="group-form-name"]').setValue('New group')
    await addRule(wrapper, 'claude-opus-*', '2')
    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()

    expect(api.createGroup).toHaveBeenCalledTimes(1)
    expect(api.replaceModelRates).toHaveBeenCalledWith(77, [{ model: 'claude-opus-*', rate_multiplier: 2 }])
    wrapper.unmount()
  })

  it('hides the editor and skips the summary in simple mode', async () => {
    authState.isSimpleMode = true
    const wrapper = mountView()
    await flushPromises()
    expect(api.listAllModelRates).not.toHaveBeenCalled()
    await openEdit(wrapper)
    expect(wrapper.find('[data-testid="group-model-rates-editor"]').exists()).toBe(false)
    expect(api.getModelRates).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
