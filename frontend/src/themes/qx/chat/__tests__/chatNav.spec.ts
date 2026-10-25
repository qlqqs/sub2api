import { beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick } from 'vue'
import { createStores } from '../../shell/__tests__/mocks'

const stores = vi.hoisted(() => ({ current: null as unknown as ReturnType<typeof import('../../shell/__tests__/mocks').createStores> }))
const status = vi.hoisted(() => ({ chat: false as boolean | 'error', get: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { get: status.get } }))
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

async function consolePaths() {
  const { useQxNav } = await import('../../shell/useQxNav')
  const sections = effectScope().run(() => useQxNav())!.sections
  await new Promise((r) => setTimeout(r, 0))
  await nextTick()
  return sections.value.find((s) => s.key === 'console')!.items.map((i) => i.path)
}

describe('对话入口（侧栏）', () => {
  beforeEach(() => {
    vi.resetModules()
    stores.current = createStores()
    status.get.mockReset().mockImplementation((url: string) => {
      if (url === '/chat/status') {
        return status.chat === 'error' ? Promise.reject(new Error('down')) : Promise.resolve({ data: { enabled: status.chat } })
      }
      return Promise.resolve({ data: { enabled: true } }) // 创作工作台开启，便于检查相对位置
    })
  })

  it('status 关闭时隐藏「对话」', async () => {
    status.chat = false
    expect(await consolePaths()).not.toContain('/chat')
    expect(status.get).toHaveBeenCalledWith('/chat/status')
  })

  it('status 请求失败按未启用处理', async () => {
    status.chat = 'error'
    expect(await consolePaths()).not.toContain('/chat')
  })

  it('status 开启时显示在「创作工作台」旁边', async () => {
    status.chat = true
    const paths = await consolePaths()
    expect(paths.slice(0, 4)).toEqual(['/dashboard', '/keys', '/chat', '/studio'])
  })
})
