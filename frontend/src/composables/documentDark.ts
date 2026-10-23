import { ref } from 'vue'

/**
 * CUSTOM: 可被 computed 追踪的深色模式判断。
 * 上游图表用 `computed(() => document.documentElement.classList.contains('dark'))` 取配色，
 * classList 不是响应式的，切换深浅色后网格线 / 文字颜色停留在页面打开时的状态。
 * 改成 `computed(() => isDocumentDark())`：一个全局 MutationObserver 监听 <html> 的 class，
 * 变化时递增版本号，让依赖它的 computed 重新计算。
 */
const version = ref(0)
let observer: MutationObserver | null = null

function ensureObserver(): void {
  if (observer || typeof MutationObserver === 'undefined' || typeof document === 'undefined') return
  observer = new MutationObserver(() => {
    version.value++
  })
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
}

export function isDocumentDark(): boolean {
  if (typeof document === 'undefined') return false
  ensureObserver()
  void version.value
  return document.documentElement.classList.contains('dark')
}
