import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ClientDownloadsContent from '../ClientDownloadsContent.vue'
import type { ClientDownload } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key}|${JSON.stringify(params)}` : key),
      te: () => true
    })
  }
})

const UA = {
  win: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/140.0 Safari/537.36',
  mac: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15',
  iphone: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148'
}

const clients: ClientDownload[] = [
  {
    id: 'codex',
    name: 'Codex',
    description: 'desktop app',
    icon_url: '',
    version: '',
    homepage_url: '',
    links: [
      { label: 'Windows x64', platform: 'windows', arch: 'x64', url: 'https://example.com/win-x64' },
      { label: 'Windows ARM64', platform: 'windows', arch: 'arm64', url: 'https://example.com/win-arm64' },
      { label: 'Intel Mac', platform: 'macos', arch: 'x64', url: 'https://example.com/mac-intel' },
      { label: 'Apple Silicon Mac', platform: 'macos', arch: 'arm64', url: 'https://example.com/mac-arm64' }
    ]
  },
  {
    id: 'cc-switch',
    name: 'CC Switch',
    description: '',
    icon_url: '',
    version: 'v4.0.20',
    homepage_url: '',
    links: [{ label: 'Windows x64', platform: 'windows', arch: 'x64', url: 'https://example.com/ccs.msi' }]
  }
]

async function mountAs(userAgent: string, list = clients) {
  vi.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue(userAgent)
  const wrapper = mount(ClientDownloadsContent, { props: { clients: list }, global: { stubs: { Icon: true } } })
  await flushPromises()
  return wrapper
}

const selectedTab = (wrapper: Awaited<ReturnType<typeof mountAs>>) =>
  wrapper.findAll('[role="tab"]').find((tab) => tab.attributes('aria-selected') === 'true')?.attributes('data-testid')

describe('ClientDownloadsContent (CUSTOM)', () => {
  afterEach(() => vi.restoreAllMocks())

  it("opens on the visitor's system and picks the matching architecture", async () => {
    const wrapper = await mountAs(UA.win)
    expect(selectedTab(wrapper)).toBe('client-downloads-tab-windows')

    const cards = wrapper.findAll('[data-testid="client-download-card"]')
    expect(cards[0].get('[data-testid="client-download-recommended"]').attributes('href')).toBe(
      'https://example.com/win-x64'
    )
    expect(cards[0].findAll('[data-testid="client-download-other"]').map((a) => a.text())).toEqual(['Windows ARM64'])
    expect(cards[1].get('[data-testid="client-download-recommended"]').attributes('href')).toBe(
      'https://example.com/ccs.msi'
    )
    expect(wrapper.find('[data-testid="client-downloads-hint"]').text()).toBe('downloads.archHints.windows')
  })

  it('switches system with the tabs and marks clients without that build', async () => {
    const wrapper = await mountAs(UA.win)
    await wrapper.get('[data-testid="client-downloads-tab-macos"]').trigger('click')

    const cards = wrapper.findAll('[data-testid="client-download-card"]')
    // 不是访客自己的系统：架构未知，Mac 默认推荐 Apple Silicon
    expect(cards[0].get('[data-testid="client-download-recommended"]').attributes('href')).toBe(
      'https://example.com/mac-arm64'
    )
    expect(cards[1].find('[data-testid="client-download-recommended"]').exists()).toBe(false)
    expect(cards[1].find('[data-testid="client-download-unavailable"]').exists()).toBe(true)
  })

  it('opens on macOS for Mac visitors', async () => {
    const wrapper = await mountAs(UA.mac)
    expect(selectedTab(wrapper)).toBe('client-downloads-tab-macos')
  })

  it('falls back to the first system and says so on unsupported devices', async () => {
    const wrapper = await mountAs(UA.iphone)
    expect(selectedTab(wrapper)).toBe('client-downloads-tab-windows')
    expect(wrapper.find('[data-testid="client-downloads-unsupported"]').exists()).toBe(true)
  })

  it('opens every download link in a new tab without an opener', async () => {
    const wrapper = await mountAs(UA.win)
    const links = wrapper.findAll('a[href^="https://example.com/"]')
    expect(links.length).toBeGreaterThan(0)
    for (const link of links) {
      expect(link.attributes('target')).toBe('_blank')
      expect(link.attributes('rel')).toBe('noopener noreferrer')
    }
  })

  it('shows an empty state without clients', async () => {
    const wrapper = await mountAs(UA.win, [])
    expect(wrapper.text()).toContain('downloads.empty')
    expect(wrapper.find('[role="tab"]').exists()).toBe(false)
  })
})
