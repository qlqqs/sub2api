<template>
  <div class="relative">
    <label v-if="label" :for="inputId" class="input-label">{{ label }}</label>
    <div
      v-if="selected"
      class="flex items-center justify-between gap-2 rounded-lg border border-primary-200 bg-primary-50 px-3 py-2 dark:border-primary-700/50 dark:bg-primary-900/20"
      :data-test="testId ? testId + '-selected' : undefined"
    >
      <span class="min-w-0 truncate text-sm text-gray-900 dark:text-white">
        {{ selected.email }}
        <span class="text-xs text-gray-500 dark:text-dark-400">({{ selected.username || '#' + selected.id }})</span>
      </span>
      <button
        type="button"
        class="shrink-0 text-lg leading-none text-gray-400 hover:text-red-600"
        :aria-label="clearLabel"
        :data-test="testId ? testId + '-clear' : undefined"
        @click="emit('clear')"
      >
        ×
      </button>
    </div>
    <template v-else>
      <input
        :id="inputId"
        v-model="query"
        type="text"
        class="input"
        autocomplete="off"
        :placeholder="placeholder"
        :aria-busy="searching ? 'true' : 'false'"
        :data-test="testId ? testId + '-input' : undefined"
        @input="scheduleSearch"
      />
      <div
        v-if="results.length > 0"
        class="absolute left-0 right-0 z-20 mt-1 max-h-48 overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-800"
        role="listbox"
      >
        <button
          v-for="user in results"
          :key="user.id"
          type="button"
          role="option"
          class="block w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-700"
          :data-test="testId ? testId + '-option' : undefined"
          @click="pick(user)"
        >
          {{ user.email }} <span class="text-xs text-gray-500">({{ user.username || '#' + user.id }})</span>
        </button>
      </div>
      <p
        v-else-if="searched && !searching && query.trim() && emptyText"
        class="mt-1 text-xs text-gray-500 dark:text-dark-400"
      >
        {{ emptyText }}
      </p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onUnmounted, ref } from 'vue'
import { affiliatesAPI, type SimpleUser } from '@/api/admin/affiliates'

const props = withDefaults(defineProps<{
  inputId: string
  label?: string
  placeholder?: string
  clearLabel?: string
  emptyText?: string
  selected: SimpleUser | null
  testId?: string
  /** Custom search source; defaults to the generic admin user lookup. */
  search?: (keyword: string, options: { signal: AbortSignal }) => Promise<SimpleUser[]>
}>(), {
  search: undefined,
  label: '',
  placeholder: '',
  clearLabel: '',
  emptyText: '',
  testId: '',
})

const emit = defineEmits<{
  select: [user: SimpleUser]
  clear: []
}>()

const query = ref('')
const results = ref<SimpleUser[]>([])
const searching = ref(false)
const searched = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
let controller: AbortController | undefined
let sequence = 0

function cancel() {
  sequence++
  if (timer) clearTimeout(timer)
  timer = undefined
  controller?.abort()
  controller = undefined
  searching.value = false
}

function scheduleSearch() {
  cancel()
  const keyword = query.value.trim()
  if (!keyword) {
    results.value = []
    searched.value = false
    return
  }
  const current = sequence
  timer = setTimeout(async () => {
    controller = new AbortController()
    searching.value = true
    try {
      const search = props.search ?? affiliatesAPI.lookupUsers
      const users = await search(keyword, { signal: controller.signal })
      if (current === sequence) results.value = users
    } catch {
      if (current === sequence) results.value = []
    } finally {
      if (current === sequence) {
        searching.value = false
        searched.value = true
      }
    }
  }, 300)
}

function pick(user: SimpleUser) {
  cancel()
  query.value = ''
  results.value = []
  searched.value = false
  emit('select', user)
}

onUnmounted(cancel)
</script>
