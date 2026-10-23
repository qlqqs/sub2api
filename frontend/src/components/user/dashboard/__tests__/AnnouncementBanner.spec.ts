import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import type { UserAnnouncement } from '@/types'

const store = vi.hoisted(() => ({
  announcements: [] as UserAnnouncement[],
  fetchAnnouncements: vi.fn(),
  markAsRead: vi.fn()
}))
const reactiveStore = reactive(store)

vi.mock('@/stores/announcements', () => ({ useAnnouncementStore: () => reactiveStore }))
const bannerApi = vi.hoisted(() => ({ getAnnouncementBanner: vi.fn() }))
vi.mock('@/api/announcementBanner', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/announcementBanner')>()),
  getAnnouncementBanner: bannerApi.getAnnouncementBanner
}))
vi.mock('@/utils/format', () => ({
  formatDateOnly: (value: string) => value.slice(0, 10),
  formatRelativeWithDateTime: (value: string) => value
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import AnnouncementBanner from '../AnnouncementBanner.vue'

function announcement(id: number, title: string, over: Partial<UserAnnouncement> = {}): UserAnnouncement {
  return {
    id,
    title,
    content: `## 说明\n\n**${title}** 详情`,
    notify_mode: 'silent',
    created_at: '2026-10-23T08:00:00Z',
    updated_at: '2026-10-23T08:00:00Z',
    ...over
  }
}

async function mountBanner(props: { variant?: 'default' | 'qx' } = {}) {
  const wrapper = mount(AnnouncementBanner, {
    props,
    global: {
      stubs: {
        Icon: { template: '<span />' },
        BaseDialog: {
          props: ['show', 'title'],
          emits: ['close'],
          template: '<div v-if="show" data-testid="dialog"><h3>{{ title }}</h3><slot /></div>'
        }
      }
    }
  })
  await flushPromises()
  return wrapper
}

const banner = '[data-testid="announcement-banner"]'
const activeTitle = (w: Awaited<ReturnType<typeof mountBanner>>) =>
  w.findAll('.banner-slide').find((s) => s.attributes('aria-hidden') === undefined)?.find('h2').text()

describe('AnnouncementBanner', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
    reactiveStore.announcements = [announcement(1, '维护通知'), announcement(2, '新模型上线', { read_at: '2026-10-23T09:00:00Z' })]
    store.fetchAnnouncements.mockReset()
    store.markAsRead.mockReset()
    bannerApi.getAnnouncementBanner.mockReset().mockResolvedValue({ order: [], hidden: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows one slide per announcement with a plain-text excerpt', async () => {
    const wrapper = await mountBanner()
    expect(store.fetchAnnouncements).toHaveBeenCalled()
    const slides = wrapper.findAll('.banner-slide')
    expect(slides.map((s) => s.find('h2').text())).toEqual(['维护通知', '新模型上线'])
    expect(slides[0].find('.banner-excerpt').text()).toBe('说明 维护通知 详情')
    expect(slides[0].text()).toContain('NEW')
    expect(slides[1].text()).not.toContain('NEW')
    expect(wrapper.findAll('[data-testid="announcement-banner-dot"]')).toHaveLength(2)
  })

  it('renders nothing when there are no announcements', async () => {
    reactiveStore.announcements = []
    expect((await mountBanner()).find(banner).exists()).toBe(false)
  })

  it('auto-advances every 4 seconds, wraps around and pauses on hover', async () => {
    const wrapper = await mountBanner()
    expect(activeTitle(wrapper)).toBe('维护通知')
    await vi.advanceTimersByTimeAsync(3999)
    expect(activeTitle(wrapper)).toBe('维护通知')
    await vi.advanceTimersByTimeAsync(1)
    expect(activeTitle(wrapper)).toBe('新模型上线')
    await vi.advanceTimersByTimeAsync(4000)
    expect(activeTitle(wrapper)).toBe('维护通知')

    await wrapper.find(banner).trigger('mouseenter')
    await vi.advanceTimersByTimeAsync(12000)
    expect(activeTitle(wrapper)).toBe('维护通知')
  })

  it('switches slides with the arrows and dots', async () => {
    const wrapper = await mountBanner()
    await wrapper.find('[data-testid="announcement-banner-prev"]').trigger('click')
    expect(activeTitle(wrapper)).toBe('新模型上线')
    await wrapper.findAll('[data-testid="announcement-banner-dot"]')[0].trigger('click')
    expect(activeTitle(wrapper)).toBe('维护通知')
  })

  it('hides carousel controls for a single announcement', async () => {
    reactiveStore.announcements = [announcement(1, '维护通知')]
    const wrapper = await mountBanner()
    expect(wrapper.find('[data-testid="announcement-banner-next"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="announcement-banner-dot"]')).toHaveLength(0)
  })

  it('opens the detail with rendered markdown and marks unread items read', async () => {
    const wrapper = await mountBanner()
    await wrapper.findAll('[data-testid="announcement-banner-detail"]')[0].trigger('click')
    const dialog = wrapper.find('[data-testid="dialog"]')
    expect(dialog.find('h3').text()).toBe('维护通知')
    expect(dialog.find('.markdown-body strong').text()).toBe('维护通知')
    expect(store.markAsRead).toHaveBeenCalledWith(1)

    store.markAsRead.mockClear()
    await wrapper.findAll('[data-testid="announcement-banner-detail"]')[1].trigger('click')
    expect(store.markAsRead).not.toHaveBeenCalled()
  })

  it('cannot be dismissed and ignores a dismissal left over in localStorage', async () => {
    localStorage.setItem('dashboard_announcement_banner_dismissed', '1,2')
    const wrapper = await mountBanner()
    expect(wrapper.find(banner).exists()).toBe(true)
    expect(wrapper.find('[data-testid="announcement-banner-dismiss"]').exists()).toBe(false)

    reactiveStore.announcements = [...reactiveStore.announcements, announcement(3, '充值活动')]
    await flushPromises()
    expect(wrapper.findAll('.banner-slide')).toHaveLength(3)
  })

  it('follows the carousel settings: hides listed ids and puts ordered ids first', async () => {
    reactiveStore.announcements = [announcement(1, '维护通知'), announcement(2, '新模型上线'), announcement(3, '充值活动'), announcement(4, '新功能')]
    bannerApi.getAnnouncementBanner.mockResolvedValue({ order: [3, 1], hidden: [2] })
    const wrapper = await mountBanner()
    // 3、1 按设置排在前；2 被隐藏；4 未设置，默认上轮播排在后面
    expect(wrapper.findAll('.banner-slide').map((s) => s.find('h2').text())).toEqual(['充值活动', '维护通知', '新功能'])
  })

  it('renders nothing when every announcement is hidden from the carousel', async () => {
    bannerApi.getAnnouncementBanner.mockResolvedValue({ order: [], hidden: [1, 2] })
    expect((await mountBanner()).find(banner).exists()).toBe(false)
  })

  it('waits for the settings before showing and falls back to all announcements on error', async () => {
    let fail: (e: Error) => void = () => {}
    bannerApi.getAnnouncementBanner.mockReturnValue(new Promise((_, reject) => (fail = reject)))
    const wrapper = mount(AnnouncementBanner, { global: { stubs: { Icon: { template: '<span />' }, BaseDialog: true } } })
    await flushPromises()
    expect(wrapper.find(banner).exists()).toBe(false)
    fail(new Error('network'))
    await flushPromises()
    expect(wrapper.findAll('.banner-slide')).toHaveLength(2)
  })

  describe('qx variant', () => {
    it('renders the line-style banner with tag, NEW marker, bell mark and progress counter', async () => {
      reactiveStore.announcements = [
        announcement(1, '维护通知'),
        announcement(2, '新模型上线', { read_at: '2026-10-23T09:00:00Z' }),
        announcement(3, '充值活动', { read_at: '2026-10-23T09:00:00Z' })
      ]
      const wrapper = await mountBanner({ variant: 'qx' })
      expect(wrapper.find(banner).classes()).toContain('qx-banner')
      expect(wrapper.find('[data-testid="announcement-banner-mark"]').exists()).toBe(true)
      expect(wrapper.find('[data-testid="announcement-banner-progress"]').text()).toBe('01 / 03')
      const slides = wrapper.findAll('.banner-slide')
      expect(slides[0].find('.qx-banner-tag').text()).toBe('announcements.title')
      expect(slides[0].find('.qx-banner-new').exists()).toBe(true)
      expect(slides[1].find('.qx-banner-new').exists()).toBe(false)
    })

    it('keeps autoplay, dots and detail behaviour without a close button', async () => {
      const wrapper = await mountBanner({ variant: 'qx' })
      await vi.advanceTimersByTimeAsync(4000)
      expect(activeTitle(wrapper)).toBe('新模型上线')
      expect(wrapper.find('[data-testid="announcement-banner-progress"]').text()).toBe('02 / 02')
      await wrapper.findAll('[data-testid="announcement-banner-dot"]')[0].trigger('click')
      expect(activeTitle(wrapper)).toBe('维护通知')
      await wrapper.findAll('[data-testid="announcement-banner-detail"]')[0].trigger('click')
      expect(store.markAsRead).toHaveBeenCalledWith(1)
      expect(wrapper.find('[data-testid="dialog"] h3').text()).toBe('维护通知')
      expect(wrapper.find('[data-testid="announcement-banner-dismiss"]').exists()).toBe(false)
    })

    it('has no progress counter for a single announcement', async () => {
      reactiveStore.announcements = [announcement(1, '维护通知')]
      const wrapper = await mountBanner({ variant: 'qx' })
      expect(wrapper.find('[data-testid="announcement-banner-progress"]').exists()).toBe(false)
      expect(wrapper.find('[data-testid="announcement-banner-dot"]').exists()).toBe(false)
    })

    it('default variant keeps the gradient banner without the line mark', async () => {
      const wrapper = await mountBanner()
      expect(wrapper.find(banner).classes()).not.toContain('qx-banner')
      expect(wrapper.find('[data-testid="announcement-banner-mark"]').exists()).toBe(false)
    })
  })
})
