/**
 * CUSTOM: 财务明细页各标签页的取数：新请求取消旧请求（AbortController），
 * 取消不当作失败；失败时保留可读原因，由页面显示「加载失败 + 重试」。
 */

import { onBeforeUnmount, ref, shallowRef, type Ref, type ShallowRef } from 'vue'
import { extractApiErrorMessage } from '@/utils/apiError'

export interface FinanceLoad<T> {
  data: ShallowRef<T | null>
  loading: Ref<boolean>
  error: Ref<string>
  load: () => Promise<void>
}

function isCanceled(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false
  const e = error as { code?: unknown; name?: unknown }
  return e.code === 'ERR_CANCELED' || e.name === 'CanceledError' || e.name === 'AbortError'
}

export function useFinanceLoad<T>(
  fetcher: (signal: AbortSignal) => Promise<T>,
  fallbackError: () => string
): FinanceLoad<T> {
  const data = shallowRef<T | null>(null)
  const loading = ref(false)
  const error = ref('')
  let controller: AbortController | null = null

  async function load() {
    controller?.abort()
    const current = new AbortController()
    controller = current
    loading.value = true
    error.value = ''
    try {
      const result = await fetcher(current.signal)
      if (controller !== current) return
      data.value = result
    } catch (err) {
      if (controller !== current || isCanceled(err)) return
      error.value = extractApiErrorMessage(err, fallbackError())
    } finally {
      if (controller === current) loading.value = false
    }
  }

  onBeforeUnmount(() => {
    controller?.abort()
    controller = null
  })

  return { data, loading, error, load }
}
