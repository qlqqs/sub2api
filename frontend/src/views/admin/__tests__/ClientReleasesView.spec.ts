import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ClientReleasesView from '../ClientReleasesView.vue'
import type { ClientRelease } from '@/api/admin'

enableAutoUnmount(afterEach)

const { listReleases, checkFile, uploadFile, stepUpRun, showError } = vi.hoisted(() => ({
  listReleases: vi.fn(),
  checkFile: vi.fn(),
  uploadFile: vi.fn(),
  stepUpRun: vi.fn((action: () => Promise<unknown>) => action()),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    clientReleases: {
      list: listReleases,
      create: vi.fn(),
      checkFile,
      uploadFile,
      updateNotes: vi.fn(),
      publish: vi.fn(),
      withdraw: vi.fn(),
      remove: vi.fn()
    }
  }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess: vi.fn() })
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ run: stepUpRun }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: (error: unknown) => (error as { cancelled?: boolean })?.cancelled === true,
  stepUpBlockReason: () => ''
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string) => key,
    te: (key: string) => key.startsWith('admin.clientReleases.errors.CLIENT_RELEASE_')
  })
}))

function draft(uploaded: string[] = []): ClientRelease {
  const names = ['qxapi-linux-amd64', 'qxapi-windows-amd64.exe']
  return {
    id: 3,
    product: 'qxapi',
    version: '0.1.1',
    notes: '',
    status: 'draft',
    published_at: null,
    created_at: '2026-10-30T00:00:00Z',
    updated_at: '2026-10-30T00:00:00Z',
    files: names.map((name) => ({ name, size: 4, sha256: 'a'.repeat(64), uploaded: uploaded.includes(name) }))
  }
}

function mountView() {
  return mount(ClientReleasesView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        BaseDialog: true,
        ConfirmDialog: true,
        Icon: true,
        TotpStepUpDialog: true
      }
    }
  })
}

async function chooseFiles(wrapper: ReturnType<typeof mountView>, names: string[]) {
  await wrapper.get('[data-testid="client-release-upload"]').trigger('click')
  const input = wrapper.get('[data-testid="client-release-files-input"]')
  Object.defineProperty(input.element, 'files', {
    value: names.map((name) => new File(['abcd'], name)),
    configurable: true
  })
  await input.trigger('change')
  await flushPromises()
}

describe('客户端版本页上传', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    stepUpRun.mockImplementation((action: () => Promise<unknown>) => action())
    listReleases.mockResolvedValue([draft()])
    checkFile.mockResolvedValue(undefined)
  })

  it('每个文件先过 check 再上传，不在 release.json 中的文件不上传', async () => {
    const calls: string[] = []
    checkFile.mockImplementation(async (_id: number, name: string) => {
      calls.push(`check ${name}`)
    })
    uploadFile.mockImplementation(async (_id: number, file: File) => {
      calls.push(`put ${file.name}`)
      return file.name === 'qxapi-linux-amd64' ? draft(['qxapi-linux-amd64']) : draft(['qxapi-linux-amd64', 'qxapi-windows-amd64.exe'])
    })
    const wrapper = mountView()
    await flushPromises()

    await chooseFiles(wrapper, ['qxapi-linux-amd64', 'notes.txt', 'qxapi-windows-amd64.exe'])

    expect(calls).toEqual([
      'check qxapi-linux-amd64',
      'put qxapi-linux-amd64',
      'check qxapi-windows-amd64.exe',
      'put qxapi-windows-amd64.exe'
    ])
    expect(wrapper.text()).toContain('notes.txt')
    expect(wrapper.text()).toContain('admin.clientReleases.notInManifest')
    expect(wrapper.get('[data-testid="client-release-publish"]').attributes('disabled')).toBeUndefined()
  })

  it('一个文件校验失败时显示原因并继续上传下一个', async () => {
    uploadFile
      .mockRejectedValueOnce({ reason: 'CLIENT_RELEASE_FILE_MISMATCH', message: 'sha256 mismatch' })
      .mockResolvedValueOnce(draft(['qxapi-windows-amd64.exe']))
    const wrapper = mountView()
    await flushPromises()

    await chooseFiles(wrapper, ['qxapi-linux-amd64', 'qxapi-windows-amd64.exe'])

    expect(uploadFile).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('admin.clientReleases.errors.CLIENT_RELEASE_FILE_MISMATCH')
    expect(wrapper.get('[data-testid="client-release-publish"]').attributes('disabled')).toBeDefined()
  })

  it('取消 TOTP 验证后停止上传，不发出文件', async () => {
    stepUpRun.mockRejectedValueOnce({ cancelled: true })
    const wrapper = mountView()
    await flushPromises()

    await chooseFiles(wrapper, ['qxapi-linux-amd64', 'qxapi-windows-amd64.exe'])

    expect(uploadFile).not.toHaveBeenCalled()
    expect(showError).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('admin.clientReleases.waiting')
  })

  it('文件没传完时不能发布', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="client-release-publish"]').attributes('disabled')).toBeDefined()
  })
})
