/**
 * CUSTOM: 创作工作台是否启用（部署时配置了 STUDIO_NOVA_URL）。
 * 结果按页面生命周期缓存一次；请求失败按未启用处理，不显示入口。
 */
import { ref } from 'vue'
import { apiClient } from '@/api/client'

const enabled = ref<boolean | undefined>(undefined)
let pending: Promise<void> | null = null

export function loadStudioEnabled(): Promise<void> {
  if (!pending) {
    pending = apiClient
      .get<{ enabled: boolean }>('/studio/status')
      .then(({ data }) => {
        enabled.value = data?.enabled === true
      })
      .catch(() => {
        enabled.value = false
      })
  }
  return pending
}

/** 侧边栏 featureFlag：未加载完成前视为未启用（opt-in）。 */
export function useStudioEnabled() {
  void loadStudioEnabled()
  return enabled
}
