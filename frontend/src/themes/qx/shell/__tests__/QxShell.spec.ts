// QxShell 给 <html> 打的页面标记：非 /admin 路由加 qx-line + data-qx-page（极简线条样式的作用范围），卸载时清掉。
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick, reactive } from 'vue'
import { mount } from '@vue/test-utils'

const route = reactive({ path: '/keys', name: 'Keys' as string | undefined })

vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ sidebarCollapsed: false }) }))
vi.mock('@/components/layout/AppHeader.vue', () => ({ default: { template: '<header />' } }))
vi.mock('../QxSidebar.vue', () => ({ default: { template: '<aside />' } }))

import QxShell from '../QxShell.vue'

const root = document.documentElement

describe('QxShell page marker', () => {
  afterEach(() => {
    route.path = '/keys'
    route.name = 'Keys'
  })

  it('marks user pages with qx-line and the route name', async () => {
    const wrapper = mount(QxShell)
    expect(root.classList.contains('qx-line')).toBe(true)
    expect(root.dataset.qxPage).toBe('Keys')

    route.path = '/admin/users'
    route.name = 'AdminUsers'
    await nextTick()
    expect(root.classList.contains('qx-line')).toBe(false)
    expect(root.dataset.qxPage).toBeUndefined()

    route.path = '/profile'
    route.name = 'Profile'
    await nextTick()
    expect(root.dataset.qxPage).toBe('Profile')

    wrapper.unmount()
    expect(root.classList.contains('qx-line')).toBe(false)
    expect(root.dataset.qxPage).toBeUndefined()
  })
})
