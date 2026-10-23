<template>
  <BaseDialog :show="show" :title="t('admin.announcements.banner.title')" width="wide" @close="emit('close')">
    <p class="mb-4 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.announcements.banner.description') }}</p>

    <div v-if="loading" class="flex justify-center py-10"><LoadingSpinner size="md" /></div>
    <div v-else-if="rows.length === 0" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">
      {{ t('admin.announcements.banner.empty') }}
    </div>
    <ol v-else class="divide-y divide-gray-100 rounded-lg border border-gray-200 dark:divide-dark-700 dark:border-dark-700" data-testid="banner-rows">
      <li
        v-for="(row, i) in rows"
        :key="row.id"
        class="flex items-center gap-3 px-3 py-2.5"
        :class="row.shown ? '' : 'opacity-60'"
        data-testid="banner-row"
        :data-id="row.id"
      >
        <label class="flex min-w-0 flex-1 cursor-pointer items-center gap-3">
          <input v-model="row.shown" type="checkbox" class="h-4 w-4 flex-shrink-0 rounded border-gray-300 text-primary-600" :aria-label="t('admin.announcements.banner.show')" data-testid="banner-toggle" />
          <span class="w-6 flex-shrink-0 text-right font-mono text-xs text-gray-400">{{ row.shown ? String(rank(i)).padStart(2, '0') : '—' }}</span>
          <span class="min-w-0 flex-1">
            <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">{{ row.title }}</span>
            <span class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">
              #{{ row.id }} · {{ t(`admin.announcements.statusLabels.${row.status}`) }}
            </span>
          </span>
        </label>
        <div class="flex flex-shrink-0 items-center gap-1">
          <button type="button" class="btn btn-secondary btn-sm px-2" :disabled="i === 0" :title="t('admin.announcements.banner.moveUp')" :aria-label="t('admin.announcements.banner.moveUp')" data-testid="banner-up" @click="move(i, -1)">
            <Icon name="chevronUp" size="sm" />
          </button>
          <button type="button" class="btn btn-secondary btn-sm px-2" :disabled="i === rows.length - 1" :title="t('admin.announcements.banner.moveDown')" :aria-label="t('admin.announcements.banner.moveDown')" data-testid="banner-down" @click="move(i, 1)">
            <Icon name="chevronDown" size="sm" />
          </button>
        </div>
      </li>
    </ol>
    <p v-if="!loading && rows.length > 0" class="mt-3 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.announcements.banner.archivedHint') }}</p>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-primary" :disabled="loading || saving" data-testid="banner-save" @click="save">
          {{ t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
/**
 * CUSTOM: 仪表盘公告轮播设置：勾选上轮播的公告并排序。
 * 列表 = 已排序的 → 其余未隐藏的（新的在前）→ 已隐藏的；保存时勾选的按列表顺序写入 order，未勾选的写入 hidden。
 */
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { applyAnnouncementBanner, getAdminAnnouncementBanner, updateAdminAnnouncementBanner } from '@/api/announcementBanner'
import type { Announcement } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'

interface Row {
  id: number
  title: string
  status: Announcement['status']
  shown: boolean
}

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const { t } = useI18n()
const appStore = useAppStore()

const rows = ref<Row[]>([])
const loading = ref(false)
const saving = ref(false)

/** 第 i 行在上轮播的公告里排第几（从 1 开始）。 */
function rank(i: number): number {
  return rows.value.slice(0, i + 1).filter((r) => r.shown).length
}

function move(i: number, delta: number) {
  const j = i + delta
  if (j < 0 || j >= rows.value.length) return
  const next = rows.value.slice()
  ;[next[i], next[j]] = [next[j], next[i]]
  rows.value = next
}

async function load() {
  loading.value = true
  try {
    const [page, config] = await Promise.all([
      adminAPI.announcements.list(1, 100, { sort_by: 'created_at', sort_order: 'desc' }),
      getAdminAnnouncementBanner()
    ])
    const items = page.items.filter((a) => a.status !== 'archived')
    const shown = applyAnnouncementBanner(items, config)
    const shownIds = new Set(shown.map((a) => a.id))
    rows.value = [
      ...shown.map((a) => ({ id: a.id, title: a.title, status: a.status, shown: true })),
      ...items.filter((a) => !shownIds.has(a.id)).map((a) => ({ id: a.id, title: a.title, status: a.status, shown: false }))
    ]
  } catch {
    appStore.showError(t('admin.announcements.banner.loadFailed'))
    rows.value = []
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await updateAdminAnnouncementBanner({
      order: rows.value.filter((r) => r.shown).map((r) => r.id),
      hidden: rows.value.filter((r) => !r.shown).map((r) => r.id)
    })
    appStore.showSuccess(t('admin.announcements.banner.saved'))
    emit('saved')
    emit('close')
  } catch (error: unknown) {
    const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    appStore.showError(detail || t('common.error'))
  } finally {
    saving.value = false
  }
}

watch(
  () => props.show,
  (open) => {
    if (open) void load()
  },
  { immediate: true }
)
</script>
