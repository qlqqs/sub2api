import { describe, expect, it } from 'vitest'
import type { ClientDownload, ClientDownloadLink } from '@/types'
import {
  availablePlatforms,
  defaultPlatform,
  detectClientEnv,
  pickRecommendedLink,
  refineClientArch,
  visibleClientDownloads
} from '../clientDownloads'

const UA = {
  win: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36',
  winArm: 'Mozilla/5.0 (Windows NT 10.0; ARM64) AppleWebKit/537.36 Chrome/140.0 Safari/537.36',
  mac: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15',
  iphone: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148',
  android: 'Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 Chrome/140.0 Mobile Safari/537.36',
  linux: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/140.0 Safari/537.36'
}

const codexLinks: ClientDownloadLink[] = [
  { label: 'Windows', platform: 'windows', arch: '', url: 'https://example.com/win' },
  { label: 'Windows x64', platform: 'windows', arch: 'x64', url: 'https://example.com/win-x64' },
  { label: 'Windows ARM64', platform: 'windows', arch: 'arm64', url: 'https://example.com/win-arm64' },
  { label: 'Apple Silicon Mac', platform: 'macos', arch: 'arm64', url: 'https://example.com/mac-arm64' },
  { label: 'Intel Mac', platform: 'macos', arch: 'x64', url: 'https://example.com/mac-intel' }
]

function client(overrides: Partial<ClientDownload> = {}): ClientDownload {
  return {
    id: 'codex',
    name: 'Codex',
    description: '',
    icon_url: '',
    version: '',
    homepage_url: '',
    links: codexLinks,
    ...overrides
  }
}

describe('detectClientEnv', () => {
  it('recognises desktop and mobile systems from the user agent', () => {
    expect(detectClientEnv({ userAgent: UA.win })).toEqual({ platform: 'windows', arch: 'x64' })
    expect(detectClientEnv({ userAgent: UA.winArm })).toEqual({ platform: 'windows', arch: 'arm64' })
    expect(detectClientEnv({ userAgent: UA.mac })).toEqual({ platform: 'macos', arch: null })
    expect(detectClientEnv({ userAgent: UA.iphone })).toEqual({ platform: 'ios', arch: 'arm64' })
    expect(detectClientEnv({ userAgent: UA.android }).platform).toBe('android')
    expect(detectClientEnv({ userAgent: UA.linux })).toEqual({ platform: 'linux', arch: 'x64' })
    expect(detectClientEnv({ userAgent: '' })).toEqual({ platform: null, arch: null })
  })

  it('treats a touch-capable "Macintosh" as an iPad', () => {
    expect(detectClientEnv({ userAgent: UA.mac, maxTouchPoints: 5 }).platform).toBe('ios')
  })
})

describe('refineClientArch', () => {
  it('uses client hints to find Apple Silicon and Windows on ARM', async () => {
    const arm = { getHighEntropyValues: async () => ({ architecture: 'arm', bitness: '64' }) }
    expect(await refineClientArch({ platform: 'macos', arch: null }, arm)).toEqual({ platform: 'macos', arch: 'arm64' })
    expect(await refineClientArch({ platform: 'windows', arch: 'x64' }, arm)).toEqual({
      platform: 'windows',
      arch: 'arm64'
    })
  })

  it('keeps the UA guess when hints are missing or fail', async () => {
    const env = { platform: 'macos' as const, arch: null }
    expect(await refineClientArch(env, undefined)).toBe(env)
    const failing = { getHighEntropyValues: () => Promise.reject(new Error('blocked')) }
    expect(await refineClientArch(env, failing)).toBe(env)
  })
})

describe('pickRecommendedLink', () => {
  it('prefers the exact architecture when it is known', () => {
    expect(pickRecommendedLink(codexLinks, { platform: 'windows', arch: 'arm64' })).toBe(2)
    expect(pickRecommendedLink(codexLinks, { platform: 'macos', arch: 'x64' })).toBe(4)
  })

  it('prefers the architecture-neutral link when the architecture is unknown', () => {
    expect(pickRecommendedLink(codexLinks, { platform: 'windows', arch: null })).toBe(0)
  })

  it('falls back to Apple Silicon on Mac and x64 elsewhere', () => {
    expect(pickRecommendedLink(codexLinks, { platform: 'macos', arch: null })).toBe(3)
    expect(pickRecommendedLink(codexLinks.slice(1), { platform: 'windows', arch: null })).toBe(0)
  })

  it('returns -1 when nothing matches the platform', () => {
    expect(pickRecommendedLink(codexLinks, { platform: 'linux', arch: 'x64' })).toBe(-1)
    expect(pickRecommendedLink(codexLinks, { platform: null, arch: null })).toBe(-1)
  })
})

describe('visibleClientDownloads', () => {
  it('is empty unless the switch is explicitly on', () => {
    expect(visibleClientDownloads(null)).toEqual([])
    expect(visibleClientDownloads({ client_downloads: [client()] })).toEqual([])
    expect(visibleClientDownloads({ client_downloads_enabled: false, client_downloads: [client()] })).toEqual([])
    expect(visibleClientDownloads({ client_downloads_enabled: true, client_downloads: [client()] })).toHaveLength(1)
  })

  it('drops unsafe URLs and clients left without links', () => {
    const result = visibleClientDownloads({
      client_downloads_enabled: true,
      client_downloads: [
        client({
          homepage_url: 'javascript:alert(1)',
          icon_url: 'data:image/png;base64,AAAA',
          links: [
            { label: 'bad', platform: 'windows', arch: '', url: 'javascript:alert(1)' },
            { label: 'ok', platform: 'windows', arch: '', url: 'https://example.com/ok' }
          ]
        }),
        client({ id: 'empty', links: [{ label: 'bad', platform: 'other', arch: '', url: 'ftp://example.com' }] })
      ]
    })
    expect(result).toHaveLength(1)
    expect(result[0].homepage_url).toBe('')
    expect(result[0].icon_url).toBe('')
    expect(result[0].links.map((link) => link.label)).toEqual(['ok'])
  })
})

describe('availablePlatforms / defaultPlatform', () => {
  const clients = [
    client(),
    client({ id: 'linux-only', links: [{ label: 'Linux', platform: 'linux', arch: '', url: 'https://example.com/l' }] })
  ]

  it('lists platforms in a fixed order regardless of configuration order', () => {
    expect(availablePlatforms(clients)).toEqual(['windows', 'macos', 'linux'])
    expect(availablePlatforms([])).toEqual([])
  })

  it("selects the visitor's own system when it has downloads, otherwise the first one", () => {
    const platforms = availablePlatforms(clients)
    expect(defaultPlatform(platforms, { platform: 'macos', arch: null })).toBe('macos')
    expect(defaultPlatform(platforms, { platform: 'ios', arch: 'arm64' })).toBe('windows')
    expect(defaultPlatform(platforms, { platform: null, arch: null })).toBe('windows')
    expect(defaultPlatform([], { platform: 'macos', arch: null })).toBeNull()
  })
})
