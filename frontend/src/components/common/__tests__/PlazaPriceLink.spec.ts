import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive } from 'vue'
import PlazaPriceLink from '../PlazaPriceLink.vue'
import { canViewModelPlaza, modelPlazaPriceRoute } from '@/utils/modelPlazaAccess'

const state = vi.hoisted(() => ({
  app: null as unknown as { cachedPublicSettings: Record<string, unknown> | null },
  auth: null as unknown as { isAuthenticated: boolean }
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => state.app }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => state.auth }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const RouterLinkStub = {
  props: ['to'],
  template: '<a data-test="link" :data-to="JSON.stringify(to)"><slot /></a>'
}

function mountLink(props: Record<string, unknown> = {}) {
  return mount(PlazaPriceLink, {
    props,
    global: { stubs: { RouterLink: RouterLinkStub, Icon: true } }
  })
}

describe('PlazaPriceLink (CUSTOM)', () => {
  beforeEach(() => {
    state.app = reactive({ cachedPublicSettings: { model_plaza_enabled: true, model_plaza_require_auth: false } })
    state.auth = reactive({ isAuthenticated: true })
  })

  it('links logged-in users to the embedded plaza focused on the group', () => {
    const link = mountLink({ groupId: 7 }).get('[data-test="link"]')
    expect(JSON.parse(link.attributes('data-to')!)).toEqual({
      path: '/model-plaza',
      query: { embedded: '1', group: '7' }
    })
    expect(link.text()).toBe('modelPlaza.priceLink.view')
  })

  it('links anonymous visitors to the standalone plaza (no embedded)', () => {
    state.auth.isAuthenticated = false
    const link = mountLink({ groupId: 7 }).get('[data-test="link"]')
    expect(JSON.parse(link.attributes('data-to')!)).toEqual({ path: '/model-plaza', query: { group: '7' } })
  })

  it('omits the group when none is given and uses a custom label', () => {
    const link = mountLink({ label: 'See all' }).get('[data-test="link"]')
    expect(JSON.parse(link.attributes('data-to')!)).toEqual({ path: '/model-plaza', query: { embedded: '1' } })
    expect(link.text()).toBe('See all')
  })

  it.each([
    ['plaza disabled', { model_plaza_enabled: false }, true],
    ['settings not loaded (opt-in)', null, true],
    ['anonymous and plaza requires sign-in', { model_plaza_enabled: true, model_plaza_require_auth: true }, false]
  ])('is hidden when %s', (_label, settings, authenticated) => {
    state.app.cachedPublicSettings = settings as Record<string, unknown> | null
    state.auth.isAuthenticated = authenticated as boolean
    expect(mountLink({ groupId: 1 }).find('[data-test="link"]').exists()).toBe(false)
  })

  it('is visible to signed-in users even when the plaza requires sign-in', () => {
    state.app.cachedPublicSettings = { model_plaza_enabled: true, model_plaza_require_auth: true }
    expect(mountLink({ groupId: 1 }).find('[data-test="link"]').exists()).toBe(true)
  })

  it('reacts to settings changes', async () => {
    const wrapper = mountLink({ groupId: 1 })
    expect(wrapper.find('[data-test="link"]').exists()).toBe(true)
    state.app.cachedPublicSettings = { model_plaza_enabled: false }
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-test="link"]').exists()).toBe(false)
  })
})

describe('modelPlazaAccess helpers', () => {
  it('mirrors the home entry check', () => {
    expect(canViewModelPlaza({ model_plaza_enabled: true }, false)).toBe(true)
    expect(canViewModelPlaza({ model_plaza_enabled: true, model_plaza_require_auth: true }, false)).toBe(false)
    expect(canViewModelPlaza({ model_plaza_enabled: true, model_plaza_require_auth: true }, true)).toBe(true)
    expect(canViewModelPlaza({ model_plaza_enabled: false }, true)).toBe(false)
    expect(canViewModelPlaza(undefined, true)).toBe(false)
  })

  it('builds the plaza route', () => {
    expect(modelPlazaPriceRoute(3, true)).toEqual({ path: '/model-plaza', query: { embedded: '1', group: '3' } })
    expect(modelPlazaPriceRoute(null, false)).toEqual({ path: '/model-plaza', query: {} })
  })
})
