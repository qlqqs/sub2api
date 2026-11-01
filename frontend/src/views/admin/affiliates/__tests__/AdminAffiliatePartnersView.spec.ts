import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'

import AdminAffiliatePartnerDetailView from '../AdminAffiliatePartnerDetailView.vue'
import AdminAffiliatePartnersView from '../AdminAffiliatePartnersView.vue'
import { normalizeAdminPartnerDetail, normalizeAdminPartnerItem } from '@/api/admin/affiliatePartners'
import { normalizePartnerMonthTotals } from '@/api/affiliatePartner'

const api = vi.hoisted(() => ({
  listPartners: vi.fn(),
  getPartner: vi.fn(),
  updatePartner: vi.fn(),
  listStatements: vi.fn()
}))
const toast = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }))

vi.mock('@/api/admin/affiliatePartners', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/admin/affiliatePartners')>()
  return { ...actual, default: api, affiliatePartnersAPI: api }
})
vi.mock('@/stores/app', () => ({ useAppStore: () => toast }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}${JSON.stringify(params)}` : key) })
  }
})

const ITEM = {
  user: { id: 75, email: '1185393366@qq.com', username: 'malaixin', deleted: false },
  region_name: '三科大',
  share_rate: '0.4000',
  next_share_rate: '0.3500',
  next_rate_month: '2026-12',
  since: '2026-11-02T10:00:00+08:00',
  agent_count: 1,
  member_count: 15,
  loss_carried: '12.5',
  unconfirmed_months: 1,
  current_month: { month: '2026-11', status: 'current', share_amount: '22.32' }
}

const Stub = defineComponent({ render: () => h('div') })

const stubs = {
  AppLayout: { template: '<main><slot /></main>' },
  TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
  DataTable: {
    props: ['columns', 'data'],
    template: `<table><tr v-for="(row, i) in data" :key="i" :data-test="'row-' + i"><td v-for="c in columns" :key="c.key" :data-test="'cell-' + c.key"><slot :name="'cell-' + c.key" :row="row" /></td></tr></table>`
  },
  Icon: true,
  PartnerCreateDialog: { props: ['show'], template: '<div v-if="show" data-test="create-dialog" />' },
  PartnerAttachDialog: { props: ['show'], template: '<div v-if="show" data-test="attach-dialog" />' },
  PartnerStatementDialog: { props: ['show', 'month', 'statements'], template: '<div v-if="show" data-test="statement-dialog">{{ month }}</div>' }
}

async function mountAt(path: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin/affiliates/partners', name: 'AdminAffiliatePartners', component: AdminAffiliatePartnersView },
      { path: '/admin/affiliates/partners/:userId', name: 'AdminAffiliatePartnerDetail', component: AdminAffiliatePartnerDetailView },
      { path: '/admin/affiliates/team/:viewerId', name: 'AdminAffiliateTeamViewer', component: Stub }
    ]
  })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(defineComponent({ render: () => h(RouterView) }), { global: { plugins: [router], stubs } })
  await flushPromises()
  return { wrapper, router }
}

describe('AdminAffiliatePartnersView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.listPartners.mockResolvedValue([normalizeAdminPartnerItem(ITEM)])
  })

  it('lists partners with rate, next rate, estimate, loss and unconfirmed months', async () => {
    const { wrapper } = await mountAt('/admin/affiliates/partners')
    const row = wrapper.get('[data-test="row-0"]')
    expect(row.get('[data-test="cell-region_name"]').text()).toBe('三科大')
    expect(row.get('[data-test="cell-share_rate"]').text()).toContain('40%')
    expect(row.get('[data-test="partner-next-rate"]').text()).toBe('partner.nextRate{"month":"2026-12","rate":"35%"}')
    expect(row.get('[data-test="cell-share"]').text()).toBe('¥22.32')
    expect(row.get('[data-test="cell-loss_carried"]').text()).toBe('¥12.50')
    expect(row.get('[data-test="cell-unconfirmed_months"]').text()).toBe('1')
    expect(row.get('[data-test="partner-detail-link"]').attributes('href')).toBe('/admin/affiliates/partners/75')

    await wrapper.get('[data-test="partners-create-open"]').trigger('click')
    expect(wrapper.find('[data-test="create-dialog"]').exists()).toBe(true)
  })
})

describe('AdminAffiliatePartnerDetailView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // 每次返回新对象，与真实接口一样触发表单回填。
    api.getPartner.mockImplementation(async () =>
      normalizeAdminPartnerDetail({
        ...ITEM,
        agents: [
          {
            user: { id: 59, email: 'a@x.com', username: 'a', deleted: false },
            attached_at: '2026-11-03T00:00:00+08:00',
            source: 'promote',
            attached_by_email: null,
            agent_level: 1,
            team: { owned: 6, via_upline: 3 },
            current_month: { revenue: '9.68' }
          }
        ]
      })
    )
    api.updatePartner.mockResolvedValue(normalizeAdminPartnerItem(ITEM))
    api.listStatements.mockResolvedValue([
      normalizePartnerMonthTotals({ month: '2026-11', status: 'current' }),
      normalizePartnerMonthTotals({ month: '2026-10', status: 'draft' })
    ])
  })

  it('prefills next month rate and sends only the changed fields', async () => {
    const { wrapper } = await mountAt('/admin/affiliates/partners/75')
    expect(api.getPartner).toHaveBeenCalledWith(75, expect.anything())
    expect(wrapper.get('[data-test="partner-detail-title"]').text()).toBe('三科大')
    expect((wrapper.get('[data-test="partner-edit-rate"]').element as HTMLInputElement).value).toBe('35')
    expect(wrapper.get('[data-test="partner-edit-submit"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="partner-edit-region"]').setValue('三科大东区')
    await wrapper.get('[data-test="partner-edit-form"]').trigger('submit')
    await flushPromises()
    expect(api.updatePartner).toHaveBeenLastCalledWith(75, { region_name: '三科大东区' })
    expect(toast.showSuccess).toHaveBeenLastCalledWith('common.saved')

    // 改回当前比例（40%）= 删除下月比例。
    await wrapper.get('[data-test="partner-edit-rate"]').setValue('40')
    await wrapper.get('[data-test="partner-edit-form"]').trigger('submit')
    await flushPromises()
    expect(api.updatePartner).toHaveBeenLastCalledWith(75, { share_rate: '0.4000' })
    expect(toast.showSuccess).toHaveBeenLastCalledWith('admin.partners.edit.savedRate')
  })

  it('shows attached managers and opens the attach dialog', async () => {
    const { wrapper } = await mountAt('/admin/affiliates/partners/75?tab=agents')
    const row = wrapper.get('[data-test="partner-agent-row"]')
    expect(row.text()).toContain('a@x.com')
    expect(row.text()).toContain('admin.partners.agents.sourcePromote')
    expect(row.text()).toContain('¥9.68')
    await wrapper.get('[data-test="partner-attach-open"]').trigger('click')
    expect(wrapper.find('[data-test="attach-dialog"]').exists()).toBe(true)
  })

  it('loads statements on the statements tab and opens one', async () => {
    const { wrapper, router } = await mountAt('/admin/affiliates/partners/75')
    expect(api.listStatements).not.toHaveBeenCalled()
    await wrapper.get('[data-test="partner-tab-statements"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ tab: 'statements' })
    expect(api.listStatements).toHaveBeenCalledWith(75, expect.anything())

    await wrapper.get('[data-test="partner-statement-2026-10"] [data-test="partner-statement-open"]').trigger('click')
    expect(wrapper.get('[data-test="statement-dialog"]').text()).toBe('2026-10')
  })
})
