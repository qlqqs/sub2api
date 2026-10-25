/**
 * CUSTOM: 客户端下载页（/downloads）的公共逻辑：可见性、访客系统识别、推荐链接。
 */
import type {
  ClientDownload,
  ClientDownloadArch,
  ClientDownloadLink,
  ClientDownloadPlatform,
  PublicSettings
} from '@/types'
import { sanitizeUrl } from '@/utils/url'

export const CLIENT_DOWNLOAD_PLATFORMS: readonly ClientDownloadPlatform[] = [
  'windows',
  'macos',
  'linux',
  'android',
  'ios',
  'other'
]
export const CLIENT_DOWNLOAD_ARCHES: readonly ClientDownloadArch[] = ['', 'x64', 'arm64', 'x86', 'universal']

/** 与后端校验上限一致，后台编辑器用来禁用「添加」按钮。 */
export const MAX_CLIENT_DOWNLOADS = 20
export const MAX_CLIENT_DOWNLOAD_LINKS = 20

/** 访客设备；识别不出时为 null。 */
export interface ClientEnv {
  platform: ClientDownloadPlatform | null
  arch: 'x64' | 'arm64' | null
}

/**
 * 页面实际展示的客户端：开关打开、链接地址合法、至少剩一个链接。
 * 旧后端没有这两个字段时视为关闭。
 */
export function visibleClientDownloads(settings: Partial<PublicSettings> | null | undefined): ClientDownload[] {
  if (!settings || settings.client_downloads_enabled !== true) return []
  const list = Array.isArray(settings.client_downloads) ? settings.client_downloads : []
  return list
    .map((client) => ({
      ...client,
      icon_url: sanitizeUrl(client.icon_url || ''),
      homepage_url: sanitizeUrl(client.homepage_url || ''),
      links: (client.links ?? [])
        .map((link) => ({ ...link, url: sanitizeUrl(link.url || '') }))
        .filter((link) => link.url !== '')
    }))
    .filter((client) => client.name && client.links.length > 0)
}

type NavigatorLike = Pick<Navigator, 'userAgent'> & { maxTouchPoints?: number }

/** 按 UA 粗略识别系统与架构。Mac 的 UA 一律写 Intel，架构留给 refineClientArch。 */
export function detectClientEnv(nav: NavigatorLike | undefined = globalThis.navigator): ClientEnv {
  const ua = nav?.userAgent || ''
  if (/iPhone|iPad|iPod/i.test(ua)) return { platform: 'ios', arch: 'arm64' }
  // iPadOS 默认请求桌面版页面，UA 与 Mac 相同，只能靠触点数区分
  if (/Macintosh/i.test(ua) && (nav?.maxTouchPoints ?? 0) > 1) return { platform: 'ios', arch: 'arm64' }
  if (/Android/i.test(ua)) return { platform: 'android', arch: null }
  if (/Windows/i.test(ua)) {
    if (/ARM64|aarch64/i.test(ua)) return { platform: 'windows', arch: 'arm64' }
    if (/Win64|x64|WOW64/i.test(ua)) return { platform: 'windows', arch: 'x64' }
    return { platform: 'windows', arch: null }
  }
  if (/Macintosh|Mac OS X/i.test(ua)) return { platform: 'macos', arch: null }
  if (/Linux|X11|CrOS/i.test(ua)) {
    if (/aarch64|arm64/i.test(ua)) return { platform: 'linux', arch: 'arm64' }
    if (/x86_64|amd64/i.test(ua)) return { platform: 'linux', arch: 'x64' }
    return { platform: 'linux', arch: null }
  }
  return { platform: null, arch: null }
}

interface UADataLike {
  getHighEntropyValues?: (hints: string[]) => Promise<{ architecture?: string; bitness?: string }>
}

/**
 * Chromium 系浏览器可查询真实 CPU 架构（Windows on ARM 的 UA 也写 x64，Apple Silicon 的 UA 写 Intel）。
 * 其他浏览器或查询失败时原样返回。
 */
export async function refineClientArch(
  env: ClientEnv,
  uaData: UADataLike | undefined = (globalThis.navigator as Navigator & { userAgentData?: UADataLike })
    ?.userAgentData
): Promise<ClientEnv> {
  if (!env.platform || env.platform === 'ios' || !uaData?.getHighEntropyValues) return env
  try {
    const { architecture, bitness } = await uaData.getHighEntropyValues(['architecture', 'bitness'])
    if (architecture === 'arm') return { ...env, arch: 'arm64' }
    if (architecture === 'x86' && bitness === '64') return { ...env, arch: 'x64' }
  } catch {
    // 用户代理客户端提示被禁用时忽略
  }
  return env
}

/**
 * 当前设备的推荐链接下标，没有匹配平台时为 -1。
 * 架构已知时优先同架构；未知时优先不区分架构的链接（通用安装包 / 自动识别），
 * 再按大多数设备的情况：Mac 选 Apple Silicon，其余选 x64。
 */
export function pickRecommendedLink(links: readonly ClientDownloadLink[], env: ClientEnv): number {
  if (!env.platform) return -1
  const candidates = links
    .map((link, index) => ({ link, index }))
    .filter(({ link }) => link.platform === env.platform)
  if (candidates.length === 0) return -1

  const byArch = (...arches: ClientDownloadArch[]) =>
    arches.map((arch) => candidates.find(({ link }) => link.arch === arch)).find(Boolean)?.index

  const preferred = env.arch
    ? byArch(env.arch, 'universal', '')
    : byArch('universal', '', env.platform === 'macos' ? 'arm64' : 'x64')
  return preferred ?? candidates[0].index
}

/** 至少有一个客户端提供下载的平台，按固定顺序排列（用作系统标签页）。 */
export function availablePlatforms(clients: readonly ClientDownload[]): ClientDownloadPlatform[] {
  const present = new Set(clients.flatMap((client) => client.links.map((link) => link.platform)))
  return CLIENT_DOWNLOAD_PLATFORMS.filter((platform) => present.has(platform))
}

/** 默认选中的系统标签：访客的系统有下载时选它，否则选第一个。 */
export function defaultPlatform(
  platforms: readonly ClientDownloadPlatform[],
  env: ClientEnv
): ClientDownloadPlatform | null {
  if (env.platform && platforms.includes(env.platform)) return env.platform
  return platforms[0] ?? null
}
