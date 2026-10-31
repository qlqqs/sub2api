<template>
  <AppLayout>
    <div class="space-y-6">
      <section
        class="flex flex-col gap-4 border-b border-gray-200 pb-5 dark:border-dark-700 sm:flex-row sm:items-end sm:justify-between"
      >
        <div class="min-w-0">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.clientReleases.title') }}
          </h2>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.clientReleases.description') }}
          </p>
          <p class="mt-2 break-all text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.clientReleases.feedUrl') }}:
            <a :href="feedUrl" target="_blank" rel="noopener" class="font-mono text-primary-600 hover:underline dark:text-primary-400">{{ feedUrl }}</a>
          </p>
        </div>

        <div class="flex flex-shrink-0 items-center gap-2">
          <input
            ref="manifestInput"
            class="hidden"
            type="file"
            accept=".json,application/json"
            data-testid="client-release-manifest-input"
            @change="handleManifestSelected"
          />
          <button
            type="button"
            class="btn btn-primary"
            :disabled="creating"
            data-testid="client-release-create"
            @click="manifestInput?.click()"
          >
            <Icon name="plus" size="sm" />
            {{ creating ? t('common.processing') : t('admin.clientReleases.create') }}
          </button>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="loading"
            :title="t('common.refresh')"
            @click="loadReleases"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="sr-only">{{ t('common.refresh') }}</span>
          </button>
        </div>
      </section>

      <div
        class="border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-800 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200"
      >
        <p>{{ t('admin.clientReleases.createHint') }}</p>
        <p class="mt-1">{{ t('admin.clientReleases.retention') }}</p>
      </div>

      <div
        v-if="loading && releases.length === 0"
        class="flex min-h-48 items-center justify-center text-sm text-gray-500"
      >
        {{ t('common.loading') }}
      </div>

      <div
        v-else-if="releases.length === 0"
        class="flex min-h-56 flex-col items-center justify-center border border-dashed border-gray-300 px-6 text-center dark:border-dark-600"
      >
        <Icon name="download" size="xl" class="text-gray-400" />
        <p class="mt-3 font-medium text-gray-800 dark:text-gray-200">
          {{ t('admin.clientReleases.empty') }}
        </p>
        <p class="mt-1 max-w-lg text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.clientReleases.emptyHint') }}
        </p>
      </div>

      <article
        v-for="release in releases"
        :key="release.id"
        class="card overflow-hidden border border-gray-200 dark:border-dark-700"
        :data-testid="`client-release-${release.version}`"
      >
        <header
          class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 p-5 dark:border-dark-700"
        >
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h3 class="font-mono text-base font-semibold text-gray-900 dark:text-white">
                {{ release.version }}
              </h3>
              <span :class="['badge', statusClass(release.status)]">
                {{ t(`admin.clientReleases.status.${release.status}`) }}
              </span>
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              <span v-if="release.published_at">{{ t('admin.clientReleases.publishedAt', { time: formatDateTime(release.published_at) }) }}</span>
              <span v-else>{{ t('admin.clientReleases.createdAt', { time: formatDateTime(release.created_at) }) }}</span>
              · {{ t('admin.clientReleases.filesProgress', { done: uploadedCount(release), total: release.files.length }) }}
            </p>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <template v-if="editable(release)">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="uploadingID !== null"
                data-testid="client-release-upload"
                @click="pickFiles(release)"
              >
                <Icon name="upload" size="sm" />
                {{ t('admin.clientReleases.uploadFiles') }}
              </button>
              <button
                type="button"
                class="btn btn-primary btn-sm"
                :disabled="!allUploaded(release) || busyID === release.id || uploadingID === release.id"
                :title="allUploaded(release) ? '' : t('admin.clientReleases.publishNeedsFiles')"
                data-testid="client-release-publish"
                @click="askPublish(release)"
              >
                {{ t('admin.clientReleases.publish') }}
              </button>
            </template>
            <button
              v-if="release.status === 'published'"
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="busyID === release.id"
              data-testid="client-release-withdraw"
              @click="askWithdraw(release)"
            >
              {{ t('admin.clientReleases.withdraw') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              data-testid="client-release-edit-notes"
              @click="openNotes(release)"
            >
              <Icon name="edit" size="sm" />
              {{ t('admin.clientReleases.editNotes') }}
            </button>
            <button
              v-if="editable(release)"
              type="button"
              class="btn btn-danger btn-sm"
              :disabled="busyID === release.id || uploadingID === release.id"
              data-testid="client-release-delete"
              @click="askDelete(release)"
            >
              <Icon name="trash" size="sm" />
              {{ t('admin.clientReleases.delete') }}
            </button>
          </div>
        </header>

        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-5 py-2 font-medium">{{ t('admin.clientReleases.file') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('admin.clientReleases.size') }}</th>
                <th class="hidden px-3 py-2 font-medium md:table-cell">{{ t('admin.clientReleases.sha256') }}</th>
                <th class="px-5 py-2 font-medium">{{ t('admin.clientReleases.fileState') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="file in release.files" :key="file.name">
                <td class="whitespace-nowrap px-5 py-2 font-mono text-gray-900 dark:text-gray-100">{{ file.name }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-gray-600 dark:text-gray-300">{{ formatBytes(file.size) }}</td>
                <td class="hidden px-3 py-2 font-mono text-xs text-gray-500 dark:text-gray-400 md:table-cell" :title="file.sha256">
                  {{ file.sha256.slice(0, 16) }}…
                </td>
                <td class="whitespace-nowrap px-5 py-2">
                  <span v-if="progressOf(release, file.name)?.error" class="text-red-600 dark:text-red-400">
                    {{ progressOf(release, file.name)?.error }}
                  </span>
                  <span v-else-if="progressOf(release, file.name)" class="text-primary-600 dark:text-primary-400">
                    {{ progressText(file, progressOf(release, file.name)!) }}
                  </span>
                  <span v-else-if="file.uploaded" class="inline-flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
                    <Icon name="check" size="sm" />{{ t('admin.clientReleases.uploaded') }}
                  </span>
                  <span v-else class="text-gray-400 dark:text-gray-500">{{ t('admin.clientReleases.notUploaded') }}</span>
                </td>
              </tr>
              <tr v-for="name in strayFiles(release)" :key="`stray-${name}`">
                <td class="whitespace-nowrap px-5 py-2 font-mono text-gray-500 line-through">{{ name }}</td>
                <td class="px-3 py-2"></td>
                <td class="hidden px-3 py-2 md:table-cell"></td>
                <td class="whitespace-nowrap px-5 py-2 text-red-600 dark:text-red-400">{{ t('admin.clientReleases.notInManifest') }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="border-t border-gray-100 px-5 py-4 dark:border-dark-700">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.clientReleases.notes') }}</p>
          <p
            v-if="release.notes"
            class="mt-1 whitespace-pre-wrap break-words text-sm text-gray-700 dark:text-gray-200"
          >{{ release.notes }}</p>
          <p v-else class="mt-1 text-sm text-gray-400 dark:text-gray-500">{{ t('admin.clientReleases.notesEmpty') }}</p>
        </div>
      </article>
    </div>

    <input
      ref="filesInput"
      class="hidden"
      type="file"
      multiple
      data-testid="client-release-files-input"
      @change="handleFilesSelected"
    />

    <BaseDialog
      :show="notesRelease !== null"
      :title="notesRelease ? `${t('admin.clientReleases.notes')} · ${notesRelease.version}` : ''"
      width="wide"
      @close="notesRelease = null"
    >
      <form id="client-release-notes-form" @submit.prevent="saveNotes">
        <textarea
          v-model="notesDraft"
          rows="12"
          class="input font-mono text-sm"
          data-testid="client-release-notes-input"
        ></textarea>
        <p class="input-hint">{{ t('admin.clientReleases.notesHint') }}</p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="notesRelease = null">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="client-release-notes-form" class="btn btn-primary" :disabled="savingNotes">
            {{ savingNotes ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="confirm !== null"
      :title="confirm?.title ?? ''"
      :message="confirm?.message ?? ''"
      :confirm-text="confirm?.confirmText ?? ''"
      :cancel-text="t('common.cancel')"
      :danger="confirm?.danger ?? false"
      @confirm="runConfirm"
      @cancel="confirm = null"
    >
      <p
        v-for="line in confirm?.extra ?? []"
        :key="line"
        class="text-sm text-gray-600 dark:text-gray-400"
      >
        {{ line }}
      </p>
    </ConfirmDialog>

    <TotpStepUpDialog :controller="stepUp" />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type ClientRelease, type ClientReleaseFile, type ClientReleaseStatus } from '@/api/admin'
import { buildApiUrl } from '@/api/client'
import { useAppStore } from '@/stores'
import { formatBytes, formatDateTime } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { isStepUpBlocked, isStepUpCancelled, stepUpBlockReason, useStepUp } from '@/composables/useStepUp'

interface FileProgress {
  loaded: number
  started: boolean
  error: string
}

interface PendingConfirm {
  id: number
  title: string
  message: string
  extra: string[]
  confirmText: string
  danger: boolean
  action: () => Promise<void>
}

const { t, te } = useI18n()
const appStore = useAppStore()
const stepUp = useStepUp()

const feedUrl = new URL(buildApiUrl('/client-releases/qxapi/latest'), window.location.origin).href
const releases = ref<ClientRelease[]>([])
const loading = ref(false)
const creating = ref(false)
const busyID = ref<number | null>(null)
const uploadingID = ref<number | null>(null)
const manifestInput = ref<HTMLInputElement | null>(null)
const filesInput = ref<HTMLInputElement | null>(null)
const uploadTarget = ref<ClientRelease | null>(null)
// 上传进度按 "版本id/文件名" 记录；上传成功的文件从这里移除，改看服务端返回的 uploaded。
const progress = reactive<Record<string, FileProgress>>({})
// 选中了但不在 release.json 中的文件名，按版本 id 记录。
const stray = reactive<Record<number, string[]>>({})
const notesRelease = ref<ClientRelease | null>(null)
const notesDraft = ref('')
const savingNotes = ref(false)
const confirm = ref<PendingConfirm | null>(null)

function errorMessage(error: unknown): string {
  const e = (error ?? {}) as { reason?: string; message?: string }
  const key = `admin.clientReleases.errors.${e.reason ?? ''}`
  if (e.reason && te(key)) return t(key)
  return e.message || t('common.unknownError')
}

function reportError(error: unknown): void {
  if (isStepUpCancelled(error)) return
  if (isStepUpBlocked(error)) {
    appStore.showError(
      stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
        ? t('stepUp.adminApiKeyForbidden')
        : t('stepUp.notEnabled')
    )
    return
  }
  appStore.showError(errorMessage(error))
}

function statusClass(status: ClientReleaseStatus): string {
  if (status === 'published') return 'badge-success'
  if (status === 'withdrawn') return 'badge-warning'
  return 'badge-gray'
}

function editable(release: ClientRelease): boolean {
  return release.status === 'draft' || release.status === 'withdrawn'
}

function uploadedCount(release: ClientRelease): number {
  return release.files.filter((f) => f.uploaded).length
}

function allUploaded(release: ClientRelease): boolean {
  return uploadedCount(release) === release.files.length
}

function progressKey(id: number, name: string): string {
  return `${id}/${name}`
}

function progressOf(release: ClientRelease, name: string): FileProgress | undefined {
  return progress[progressKey(release.id, name)]
}

function progressText(file: ClientReleaseFile, p: FileProgress): string {
  if (!p.started) return t('admin.clientReleases.waiting')
  const percent = file.size > 0 ? Math.floor((p.loaded / file.size) * 100) : 100
  return t('admin.clientReleases.uploading', { percent: Math.min(percent, 100) })
}

function strayFiles(release: ClientRelease): string[] {
  return stray[release.id] ?? []
}

function replaceRelease(updated: ClientRelease): void {
  const index = releases.value.findIndex((r) => r.id === updated.id)
  if (index >= 0) releases.value[index] = updated
}

async function loadReleases(): Promise<void> {
  loading.value = true
  try {
    releases.value = await adminAPI.clientReleases.list()
  } catch (error: unknown) {
    reportError(error)
  } finally {
    loading.value = false
  }
}

async function handleManifestSelected(event: Event): Promise<void> {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = ''
  if (!file) return
  const text = await file.text()
  try {
    JSON.parse(text)
  } catch {
    appStore.showError(t('admin.clientReleases.invalidManifest'))
    return
  }
  creating.value = true
  try {
    const release = await stepUp.run(() => adminAPI.clientReleases.create(text))
    appStore.showSuccess(t('admin.clientReleases.created', { version: release.version }))
    await loadReleases()
  } catch (error: unknown) {
    reportError(error)
  } finally {
    creating.value = false
  }
}

function pickFiles(release: ClientRelease): void {
  uploadTarget.value = release
  filesInput.value?.click()
}

async function handleFilesSelected(event: Event): Promise<void> {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? [])
  target.value = ''
  const release = uploadTarget.value
  if (!release || files.length === 0) return

  const names = new Set(release.files.map((f) => f.name))
  stray[release.id] = files.filter((f) => !names.has(f.name)).map((f) => f.name)
  const queue = files.filter((f) => names.has(f.name))
  for (const file of queue) {
    progress[progressKey(release.id, file.name)] = { loaded: 0, started: false, error: '' }
  }

  uploadingID.value = release.id
  let done = 0
  try {
    for (const file of queue) {
      const key = progressKey(release.id, file.name)
      const entry = progress[key]
      try {
        // 先确认 step-up 授权仍有效，再发出整个文件（见 design.md「管理接口返回格式」）。
        await stepUp.run(() => adminAPI.clientReleases.checkFile(release.id, file.name))
        entry.started = true
        const updated = await adminAPI.clientReleases.uploadFile(release.id, file, (loaded) => {
          entry.loaded = loaded
        })
        replaceRelease(updated)
        delete progress[key]
        done++
      } catch (error: unknown) {
        if (isStepUpCancelled(error) || isStepUpBlocked(error)) {
          for (const f of queue) delete progress[progressKey(release.id, f.name)]
          reportError(error)
          return
        }
        entry.error = errorMessage(error)
      }
    }
    if (done > 0) appStore.showSuccess(t('admin.clientReleases.uploadDone', { count: done }))
  } finally {
    uploadingID.value = null
  }
}

function openNotes(release: ClientRelease): void {
  notesRelease.value = release
  notesDraft.value = release.notes
}

async function saveNotes(): Promise<void> {
  const release = notesRelease.value
  if (!release) return
  savingNotes.value = true
  try {
    replaceRelease(await adminAPI.clientReleases.updateNotes(release.id, notesDraft.value))
    notesRelease.value = null
    appStore.showSuccess(t('admin.clientReleases.notesSaved'))
  } catch (error: unknown) {
    reportError(error)
  } finally {
    savingNotes.value = false
  }
}

function askPublish(release: ClientRelease): void {
  const extra = release.notes.trim() ? [] : [t('admin.clientReleases.publishNotesEmpty')]
  extra.push(t('admin.clientReleases.retention'))
  confirm.value = {
    id: release.id,
    title: t('admin.clientReleases.publishTitle', { version: release.version }),
    message: t('admin.clientReleases.publishConfirm', { version: release.version }),
    extra,
    confirmText: t('admin.clientReleases.publish'),
    danger: false,
    action: async () => {
      const result = await stepUp.run(() => adminAPI.clientReleases.publish(release.id))
      appStore.showSuccess(
        result.removed_versions.length > 0
          ? t('admin.clientReleases.publishedRemoved', {
              version: release.version,
              removed: result.removed_versions.join(', ')
            })
          : t('admin.clientReleases.published', { version: release.version })
      )
    }
  }
}

function askWithdraw(release: ClientRelease): void {
  confirm.value = {
    id: release.id,
    title: t('admin.clientReleases.withdrawTitle', { version: release.version }),
    message: t('admin.clientReleases.withdrawConfirm', { version: release.version }),
    extra: [],
    confirmText: t('admin.clientReleases.withdraw'),
    danger: true,
    action: async () => {
      await stepUp.run(() => adminAPI.clientReleases.withdraw(release.id))
      appStore.showSuccess(t('admin.clientReleases.withdrawn', { version: release.version }))
    }
  }
}

function askDelete(release: ClientRelease): void {
  confirm.value = {
    id: release.id,
    title: t('admin.clientReleases.deleteTitle', { version: release.version }),
    message: t('admin.clientReleases.deleteConfirm', { version: release.version }),
    extra: [],
    confirmText: t('admin.clientReleases.delete'),
    danger: true,
    action: async () => {
      await stepUp.run(() => adminAPI.clientReleases.remove(release.id))
      delete stray[release.id]
      appStore.showSuccess(t('admin.clientReleases.deleted', { version: release.version }))
    }
  }
}

// 发布、撤回、删除都会影响其他版本（发布会清理旧版本），完成后整表重新加载。
async function runConfirm(): Promise<void> {
  const pending = confirm.value
  if (!pending) return
  confirm.value = null
  busyID.value = pending.id
  try {
    await pending.action()
    await loadReleases()
  } catch (error: unknown) {
    reportError(error)
  } finally {
    busyID.value = null
  }
}

onMounted(loadReleases)
</script>
