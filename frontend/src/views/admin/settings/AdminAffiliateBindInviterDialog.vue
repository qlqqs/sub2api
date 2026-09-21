<template>
  <BaseDialog :show="show" :title="t('admin.settings.features.affiliate.bind.title')" @close="close">
    <form class="space-y-4" @submit.prevent="submit">
      <UserPicker
        id="affiliate-bind-invitee"
        :label="t('admin.settings.features.affiliate.bind.invitee')"
        :placeholder="t('admin.settings.features.affiliate.bind.searchPlaceholder')"
        :selected="invitee"
        :excluded-id="inviter?.id"
        @select="selectInvitee"
        @clear="invitee = null"
      />
      <UserPicker
        id="affiliate-bind-inviter"
        :label="t('admin.settings.features.affiliate.bind.inviter')"
        :placeholder="t('admin.settings.features.affiliate.bind.searchPlaceholder')"
        :selected="inviter"
        :excluded-id="invitee?.id"
        @select="selectInviter"
        @clear="inviter = null"
      />
      <div>
        <label for="affiliate-bind-reason" class="input-label">{{ t('admin.settings.features.affiliate.bind.reason') }}</label>
        <textarea
          id="affiliate-bind-reason"
          v-model="reason"
          class="input min-h-24"
          maxlength="500"
          required
          :placeholder="t('admin.settings.features.affiliate.bind.reasonPlaceholder')"
        />
        <p class="mt-1 text-right text-xs text-gray-400">{{ reason.length }}/500</p>
      </div>
      <p class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-200">
        {{ t('admin.settings.features.affiliate.bind.warning') }}
      </p>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    </form>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">{{ t('common.cancel') }}</button>
      <button type="button" class="btn btn-primary" :disabled="!canSubmit || saving" @click="submit">
        {{ saving ? t('common.saving') : t('admin.settings.features.affiliate.bind.submit') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onUnmounted, ref, watch, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { affiliatesAPI, type SimpleUser } from '@/api/admin/affiliates'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: []; success: [] }>()
const { t } = useI18n()
const invitee = ref<SimpleUser | null>(null)
const inviter = ref<SimpleUser | null>(null)
const reason = ref('')
const saving = ref(false)
const error = ref('')

const canSubmit = computed(() => !!invitee.value && !!inviter.value && invitee.value.id !== inviter.value.id && reason.value.trim().length > 0)

watch(() => props.show, (show) => {
  if (show) {
    invitee.value = null
    inviter.value = null
    reason.value = ''
    error.value = ''
  }
})

function selectInvitee(user: SimpleUser) {
  invitee.value = user
  if (inviter.value?.id === user.id) inviter.value = null
}

function selectInviter(user: SimpleUser) {
  inviter.value = user
  if (invitee.value?.id === user.id) invitee.value = null
}

function close() {
  if (!saving.value) emit('close')
}

async function submit() {
  if (!canSubmit.value || saving.value || !invitee.value || !inviter.value) return
  saving.value = true
  error.value = ''
  try {
    await affiliatesAPI.bindInviter(invitee.value.id, { inviter_id: inviter.value.id, reason: reason.value.trim() })
    emit('success')
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('common.error'))
  } finally {
    saving.value = false
  }
}

const UserPicker = defineComponent({
  props: {
    id: { type: String, required: true },
    label: { type: String, required: true },
    placeholder: { type: String, required: true },
    selected: { type: Object as PropType<SimpleUser | null>, default: null },
    excludedId: { type: Number, default: undefined },
  },
  emits: ['select', 'clear'],
  setup(p, { emit: emitPicker }) {
    const query = ref('')
    const results = ref<SimpleUser[]>([])
    const loading = ref(false)
    let timer: number | undefined
    let requestSequence = 0
    let controller: AbortController | undefined

    function cancelSearch() {
      requestSequence++
      window.clearTimeout(timer)
      controller?.abort()
      controller = undefined
      loading.value = false
    }

    async function search() {
      cancelSearch()
      const keyword = query.value.trim()
      if (!keyword) {
        results.value = []
        return
      }
      const sequence = requestSequence
      timer = window.setTimeout(async () => {
        controller = new AbortController()
        loading.value = true
        try {
          const users = await affiliatesAPI.lookupUsers(keyword, { signal: controller.signal })
          if (sequence === requestSequence) {
            results.value = users.filter(user => user.id !== p.excludedId)
          }
        } catch {
          if (sequence === requestSequence) results.value = []
        } finally {
          if (sequence === requestSequence) loading.value = false
        }
      }, 250)
    }
    onUnmounted(cancelSearch)
    return () => h('div', {}, [
      h('label', { for: p.id, class: 'input-label' }, p.label),
      p.selected
        ? h('div', { class: 'flex items-center justify-between rounded-md border border-gray-200 px-3 py-2 dark:border-dark-700' }, [
            h('span', { class: 'text-sm' }, `${p.selected.email} (${p.selected.username || '#' + p.selected.id})`),
            h('button', { type: 'button', class: 'text-gray-500 hover:text-red-600', 'aria-label': t('admin.settings.features.affiliate.bind.clearSelection'), onClick: () => emitPicker('clear') }, '×'),
          ])
        : h('div', {}, [
            h('input', { id: p.id, value: query.value, class: 'input', placeholder: p.placeholder, autocomplete: 'off', 'aria-busy': loading.value, onInput: (e: Event) => { query.value = (e.target as HTMLInputElement).value; void search() } }),
            loading.value ? h('p', { class: 'mt-1 text-xs text-gray-500' }, t('common.loading')) : null,
            results.value.length ? h('div', { class: 'mt-1 max-h-40 overflow-y-auto rounded border border-gray-200 dark:border-dark-700' }, results.value.map(user =>
              h('button', { type: 'button', class: 'block w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-800', onClick: () => { cancelSearch(); emitPicker('select', user); query.value = ''; results.value = [] } }, `${user.email} (${user.username || '#' + user.id})`)
            )) : null,
          ]),
    ])
  },
})
</script>
