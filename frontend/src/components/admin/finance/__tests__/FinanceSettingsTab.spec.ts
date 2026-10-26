import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import FinanceSettingsTab from '../FinanceSettingsTab.vue'
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
async function setup(query: string) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/finance', component: defineComponent({ components: { FinanceSettingsTab }, template: '<FinanceSettingsTab :refresh-token="3"><template #history><div data-testid="history-slot">history integration</div></template></FinanceSettingsTab>' }) }] })
  await router.push(`/finance?tab=settings${query}`)
  await router.isReady()
  const wrapper = mount(RouterView, { global: { plugins: [router], stubs: {
    FinanceAccountCostSettings: true, FinancePaymentSettings: true,
    FinanceExpensesTab: { props: ['start', 'end'], template: '<div data-testid="expenses">{{start}}~{{end}}</div>' },
    DateRangePicker: { props: ['startDate', 'endDate'], emits: ['update:startDate', 'update:endDate'], template: '<input data-testid="expense-start" :value="startDate" @input="$emit(\'update:startDate\', $event.target.value)" />' }
  } } })
  await flushPromises(); return { wrapper, router }
}
describe('Finance settings navigation', () => {
  it('restores payment section and persists history navigation without invoking report controls', async () => {
    const { wrapper, router } = await setup('&section=payment')
    expect(wrapper.findComponent({ name: 'FinancePaymentSettings' }).exists()).toBe(true)
    await wrapper.get('[data-testid="finance-settings-history"]').trigger('click'); await flushPromises()
    expect(router.currentRoute.value.query.section).toBe('history')
    expect(wrapper.get('[data-testid="history-slot"]').text()).toContain('history integration')
    expect(wrapper.find('[data-testid="expense-start"]').exists()).toBe(false)
  })
  it('uses independent expense dates, persists changes and rejects inverted dates', async () => {
    const { wrapper, router } = await setup('&section=expenses&expense_start=2026-09-01&expense_end=2026-09-30&start=2026-01-01')
    expect(wrapper.get('[data-testid="expenses"]').text()).toBe('2026-09-01~2026-09-30')
    await wrapper.get('[data-testid="expense-start"]').setValue('2026-09-15'); await flushPromises()
    expect(router.currentRoute.value.query.expense_start).toBe('2026-09-15')
    expect(router.currentRoute.value.query.start).toBe('2026-01-01')
    await wrapper.get('[data-testid="expense-start"]').setValue('2026-10-01'); await flushPromises()
    expect(wrapper.text()).toContain('admin.finance.toolbar.rangeInvalid')
    expect(wrapper.get('[data-testid="expenses"]').text()).toBe('2026-09-15~2026-09-30')
  })
})
