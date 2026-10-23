/**
 * 漂移守卫：上游 AppSidebar.vue 里出现的每个菜单路径，都必须出现在 QxAPI 外壳菜单里。
 * 上游合并进新菜单后这里会失败，提醒同步 useQxNav.ts。
 */
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it, vi } from 'vitest'
import { effectScope } from 'vue'
import { createStores } from './mocks'

const stores = vi.hoisted(() => ({ current: null as unknown as ReturnType<typeof import('./mocks').createStores> }))

vi.mock('@/stores', () => ({
  useAppStore: () => stores.current.appStore,
  useAuthStore: () => stores.current.authStore,
  useAdminSettingsStore: () => stores.current.adminSettingsStore
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => stores.current.appStore }))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: stores.current.canUseBatchImage, refreshBatchImageAccess: vi.fn() })
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import { collectNavPaths, useQxNav } from '../useQxNav'

const upstreamSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../../../../components/layout/AppSidebar.vue'),
  'utf8'
)

/**
 * 明确忽略的上游路径（目前为空）。上游若出现外壳有意不提供的入口，在这里加上并写明原因。
 */
const IGNORED_UPSTREAM_PATHS: readonly string[] = []

function allPaths(admin: boolean): string[] {
  stores.current = createStores()
  stores.current.authStore.isAdmin = admin
  // 打开所有开关，取完整菜单
  stores.current.appStore.cachedPublicSettings = {
    available_channels_enabled: true,
    affiliate_enabled: true,
    risk_control_enabled: true,
    plugin_management_enabled: true
  }
  const scope = effectScope()
  const sections = scope.run(() => useQxNav().sections.value)!
  scope.stop()
  return collectNavPaths(sections)
}

describe('useQxNav drift guard', () => {
  it('covers every static path declared in upstream AppSidebar.vue', () => {
    const upstream = [...upstreamSource.matchAll(/path: '(\/[^']*)'/g)].map((m) => m[1])
    expect(upstream.length).toBeGreaterThan(30)
    const ours = new Set([...allPaths(false), ...allPaths(true)])
    const missing = [...new Set(upstream)].filter((p) => !ours.has(p) && !IGNORED_UPSTREAM_PATHS.includes(p))
    expect(missing).toEqual([])
  })
})
