import { describe, expect, it } from 'vitest'
import {
  buildMagpieImportDeeplink,
  magpieProviderId,
  resolveMagpieImportEndpoints
} from '@/utils/magpieImport'

function paramsFromDeeplink(deeplink: string): URLSearchParams {
  const query = deeplink.split('?')[1] || ''
  return new URLSearchParams(query)
}

describe('magpieImport utils', () => {
  const baseInput = {
    baseUrl: 'https://api.example.com',
    providerName: '示例站 · 我的密钥',
    providerId: 'api.example.com-key-12',
    apiKey: 'sk-test'
  }

  it('builds a magpie://import link Magpie can parse', () => {
    const deeplink = buildMagpieImportDeeplink({ ...baseInput, platform: 'anthropic' })
    expect(deeplink.startsWith('magpie://import?')).toBe(true)

    const params = paramsFromDeeplink(deeplink)
    expect(params.get('name')).toBe('示例站 · 我的密钥')
    expect(params.get('id')).toBe('api-example-com-key-12')
    expect(params.get('anthropic')).toBe('https://api.example.com')
    expect(params.get('key')).toBe('sk-test')
    expect(params.has('responses')).toBe(false)
    expect(params.has('chat')).toBe(false)
    expect(params.has('gemini')).toBe(false)
  })

  it.each([
    ['anthropic', { anthropic: 'https://api.example.com' }],
    ['typesafe', { anthropic: 'https://api.example.com' }],
    ['openai', { responses: 'https://api.example.com/v1', chat: 'https://api.example.com/v1' }],
    ['grok', { responses: 'https://api.example.com/v1', chat: 'https://api.example.com/v1' }],
    ['deepseek', { responses: 'https://api.example.com/v1', chat: 'https://api.example.com/v1' }],
    ['gemini', { gemini: 'https://api.example.com/v1beta' }],
    [
      'antigravity',
      {
        anthropic: 'https://api.example.com/antigravity',
        gemini: 'https://api.example.com/antigravity/v1beta'
      }
    ],
    [
      'composite',
      {
        anthropic: 'https://api.example.com',
        responses: 'https://api.example.com/v1',
        chat: 'https://api.example.com/v1',
        gemini: 'https://api.example.com/v1beta'
      }
    ]
  ])('maps %s groups to the endpoints the gateway serves', (platform, endpoints) => {
    expect(resolveMagpieImportEndpoints(platform, 'https://api.example.com')).toEqual(endpoints)
  })

  it('treats ungrouped keys as Anthropic', () => {
    expect(resolveMagpieImportEndpoints(null, 'https://api.example.com')).toEqual({
      anthropic: 'https://api.example.com'
    })
  })

  it.each(['https://api.example.com/', 'https://api.example.com/v1', 'https://api.example.com/v1/'])(
    'normalizes base URL %s to the site root',
    (baseUrl) => {
      expect(resolveMagpieImportEndpoints('openai', baseUrl)).toEqual({
        responses: 'https://api.example.com/v1',
        chat: 'https://api.example.com/v1'
      })
      expect(resolveMagpieImportEndpoints('anthropic', baseUrl)).toEqual({
        anthropic: 'https://api.example.com'
      })
    }
  )

  it('passes the site pages only when they are https', () => {
    const https = paramsFromDeeplink(
      buildMagpieImportDeeplink({
        ...baseInput,
        website: 'https://example.com',
        keysUrl: 'https://example.com/keys'
      })
    )
    expect(https.get('website')).toBe('https://example.com')
    expect(https.get('keys')).toBe('https://example.com/keys')

    const http = paramsFromDeeplink(
      buildMagpieImportDeeplink({
        ...baseInput,
        website: 'http://127.0.0.1:28080',
        keysUrl: 'http://127.0.0.1:28080/keys'
      })
    )
    expect(http.has('website')).toBe(false)
    expect(http.has('keys')).toBe(false)
  })

  it('makes provider ids of lowercase letters, digits and dashes', () => {
    expect(magpieProviderId('API.Example.com-key-12')).toBe('api-example-com-key-12')
    expect(magpieProviderId('--127.0.0.1-key-3--')).toBe('127-0-0-1-key-3')
  })
})
