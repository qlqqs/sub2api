/**
 * CUSTOM: 对话工作台是否启用（部署时配置了 CHAT_WEBUI_URL）。
 * 结果按页面生命周期缓存一次；请求失败按未启用处理，不显示入口。
 */
import { ref } from 'vue'
import { apiClient } from '@/api/client'

const enabled = ref<boolean | undefined>(undefined)
let pending: Promise<void> | null = null

export function loadChatEnabled(): Promise<void> {
  if (!pending) {
    pending = apiClient
      .get<{ enabled: boolean }>('/chat/status')
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
export function useChatEnabled() {
  void loadChatEnabled()
  return enabled
}
