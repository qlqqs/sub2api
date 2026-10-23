import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { Announcement } from '@/types'

import AnnouncementBannerDialog from '../AnnouncementBannerDialog.vue'

const { list, getConfig, updateConfig, showError, showSuccess } = vi.hoisted(() => ({
  list: vi.fn(),
  getConfig: vi.fn(),
  updateConfig: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({ adminAPI: { announcements: { list } } }))
vi.mock('@/api/announcementBanner', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/announcementBanner')>()),
  getAdminAnnouncementBanner: getConfig,
  updateAdminAnnouncementBanner: updateConfig
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function ann(id: number, status: Announcement['status'] = 'active'): Announcement {
  return {
    id,
    title: `公告 ${id}`,
    content: '',
    status,
    notify_mode: 'silent',
    targeting: { any_of: [] },
    created_at: '2026-10-23T08:00:00Z',
    updated_at: '2026-10-23T08:00:00Z'
  } as Announcement
}

async function open() {
  const wrapper = mount(AnnouncementBannerDialog, {
    props: { show: true },
    global: {
      stubs: {
        Icon: true,
        LoadingSpinner: true,
        BaseDialog: { props: ['show', 'title'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }
      }
    }
  })
  await flushPromises()
  return wrapper
}

const rowIds = (w: Awaited<ReturnType<typeof open>>) => w.findAll('[data-testid="banner-row"]').map((r) => Number(r.attributes('data-id')))
const checked = (w: Awaited<ReturnType<typeof open>>) =>
  w.findAll<HTMLInputElement>('[data-testid="banner-toggle"]').map((i) => i.element.checked)

describe('AnnouncementBannerDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    list.mockResolvedValue({ items: [ann(5), ann(4, 'draft'), ann(3), ann(2, 'archived'), ann(1)], total: 5, page: 1, page_size: 100, pages: 1 })
    getConfig.mockResolvedValue({ order: [1], hidden: [3] })
    updateConfig.mockImplementation(async (cfg) => cfg)
  })

  it('lists ordered first, then the other shown ones, then hidden; archived are left out', async () => {
    const wrapper = await open()
    expect(list).toHaveBeenCalledWith(1, 100, { sort_by: 'created_at', sort_order: 'desc' })
    expect(rowIds(wrapper)).toEqual([1, 5, 4, 3])
    expect(checked(wrapper)).toEqual([true, true, true, false])
  })

  it('saves the ticked rows in list order and the rest as hidden', async () => {
    const wrapper = await open()
    await wrapper.findAll('[data-testid="banner-down"]')[0].trigger('click') // 1 ↓ → 5,1,4,3
    await wrapper.findAll('[data-testid="banner-toggle"]')[2].setValue(false) // 4 下轮播
    await wrapper.findAll('[data-testid="banner-toggle"]')[3].setValue(true) // 3 上轮播
    await wrapper.find('[data-testid="banner-save"]').trigger('click')
    await flushPromises()
    expect(updateConfig).toHaveBeenCalledWith({ order: [5, 1, 3], hidden: [4] })
    expect(showSuccess).toHaveBeenCalled()
    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('disables moving the first row up and the last row down', async () => {
    const wrapper = await open()
    expect(wrapper.findAll('[data-testid="banner-up"]')[0].attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('[data-testid="banner-down"]')[3].attributes('disabled')).toBeDefined()
  })

  it('reports a load failure', async () => {
    getConfig.mockRejectedValue(new Error('boom'))
    const wrapper = await open()
    expect(showError).toHaveBeenCalledWith('admin.announcements.banner.loadFailed')
    expect(rowIds(wrapper)).toEqual([])
  })
})
