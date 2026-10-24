import type { GroupPlatform } from '@/types'
import { getPlatformSpec } from '@/constants/platformCatalog'

/**
 * Magpie import links (magpie://import?…, parsed by Magpie's
 * provider.ParseImport). Each endpoint parameter is a base URL Magpie appends
 * the protocol path to:
 *
 *   anthropic  the root (Magpie adds /v1/messages)
 *   responses  …/v1    (Magpie adds /responses)
 *   chat       …/v1    (Magpie adds /chat/completions)
 *   gemini     …/v1beta
 *
 * The link never saves anything by itself: Magpie shows the provider and asks
 * the user first. Magpie only accepts https endpoints, or plain http to a
 * local / private address.
 */
export interface MagpieImportEndpoints {
  anthropic?: string
  responses?: string
  chat?: string
  gemini?: string
}

export interface MagpieImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  providerName: string
  providerId: string
  apiKey: string
  /** The site's own pages; Magpie keeps them only when they are https. */
  website?: string
  keysUrl?: string
}

function withoutTrailingSlashes(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, '')
}

/** The site root: api_base_url may be configured with a trailing /v1. */
function siteRoot(baseUrl: string): string {
  return withoutTrailingSlashes(baseUrl).replace(/\/v1$/, '')
}

/**
 * Endpoints this site serves for a key, by the gateway its group's platform
 * routes through (see backend routes/gateway.go). Composite groups route per
 * model across every protocol, so they get all of them.
 */
export function resolveMagpieImportEndpoints(
  platform: GroupPlatform | undefined | null,
  baseUrl: string
): MagpieImportEndpoints {
  const root = siteRoot(baseUrl)
  if (platform === 'composite') {
    return {
      anthropic: root,
      responses: `${root}/v1`,
      chat: `${root}/v1`,
      gemini: `${root}/v1beta`
    }
  }
  switch (getPlatformSpec(platform || 'anthropic')?.gateway) {
    case 'antigravity':
      return {
        anthropic: `${root}/antigravity`,
        gemini: `${root}/antigravity/v1beta`
      }
    case 'openai':
      return {
        responses: `${root}/v1`,
        chat: `${root}/v1`
      }
    case 'gemini':
      return { gemini: `${root}/v1beta` }
    default:
      return { anthropic: root }
  }
}

/** Lowercase letters, digits and dashes, as Magpie's provider ids are. */
export function magpieProviderId(raw: string): string {
  return raw
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

export function buildMagpieImportDeeplink(input: MagpieImportDeeplinkInput): string {
  const endpoints = resolveMagpieImportEndpoints(input.platform, input.baseUrl)
  const entries: [string, string][] = [
    ['name', input.providerName],
    ['id', magpieProviderId(input.providerId)]
  ]
  for (const k of ['anthropic', 'responses', 'chat', 'gemini'] as const) {
    const url = endpoints[k]
    if (url) entries.push([k, url])
  }
  entries.push(['key', input.apiKey])
  if (input.website?.startsWith('https://')) entries.push(['website', input.website])
  if (input.keysUrl?.startsWith('https://')) entries.push(['keys', input.keysUrl])

  return `magpie://import?${new URLSearchParams(entries).toString()}`
}
