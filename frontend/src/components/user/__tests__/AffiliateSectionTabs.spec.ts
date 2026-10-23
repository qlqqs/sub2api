import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'

import AffiliateSectionTabs from '../AffiliateSectionTabs.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

const Stub = defineComponent({ render: () => h('div') })

async function mountAt(path: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/affiliate', name: 'Affiliate', component: Stub },
      { path: '/affiliate/team', name: 'AffiliateTeam', component: Stub },
      { path: '/affiliate/team/agents/:id', name: 'AffiliateTeamAgent', component: Stub }
    ]
  })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(AffiliateSectionTabs, {
    global: { plugins: [router] },
    slots: { default: '<button data-test="right-slot">x</button>' }
  })
  await flushPromises()
  return { wrapper, router }
}

function tab(wrapper: Awaited<ReturnType<typeof mountAt>>['wrapper'], key: 'invite' | 'team') {
  return wrapper.get(`[data-test="affiliate-section-tab-${key}"]`)
}

describe('AffiliateSectionTabs', () => {
  it('marks 邀请 as the current page on /affiliate', async () => {
    const { wrapper } = await mountAt('/affiliate')

    expect(tab(wrapper, 'invite').attributes('aria-current')).toBe('page')
    expect(tab(wrapper, 'invite').classes()).toContain('bg-white')
    expect(tab(wrapper, 'team').attributes('aria-current')).toBeUndefined()
    expect(tab(wrapper, 'team').attributes('href')).toBe('/affiliate/team')
    expect(tab(wrapper, 'team').text()).toBe('affiliate.team.title')
    expect(tab(wrapper, 'invite').text()).toBe('affiliate.tabs.invite')
  })

  it('marks 团队消费 as the current page on team routes', async () => {
    const { wrapper } = await mountAt('/affiliate/team')

    expect(tab(wrapper, 'team').attributes('aria-current')).toBe('page')
    expect(tab(wrapper, 'invite').attributes('aria-current')).toBeUndefined()
    expect(tab(wrapper, 'invite').attributes('href')).toBe('/affiliate')
  })

  it('switches the active tab when navigating and renders the right-side slot', async () => {
    const { wrapper, router } = await mountAt('/affiliate')
    expect(wrapper.find('[data-test="right-slot"]').exists()).toBe(true)

    await tab(wrapper, 'team').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('AffiliateTeam')
    expect(tab(wrapper, 'team').attributes('aria-current')).toBe('page')
  })
})
