/**
 * CUSTOM: 仪表盘公告轮播的名单与顺序。
 * order 里的公告按顺序排在最前；hidden 里的不上轮播；两边都没有的（如新发布的）默认上轮播、排在后面。
 */

import { apiClient } from './client'

export interface AnnouncementBannerConfig {
  order: number[]
  hidden: number[]
}

/** 用户端：只读。 */
export async function getAnnouncementBanner(): Promise<AnnouncementBannerConfig> {
  const { data } = await apiClient.get<AnnouncementBannerConfig>('/announcement-banner')
  return data
}

/** 管理端读取。 */
export async function getAdminAnnouncementBanner(): Promise<AnnouncementBannerConfig> {
  const { data } = await apiClient.get<AnnouncementBannerConfig>('/admin/settings/announcement-banner')
  return data
}

/** 管理端保存，返回去重后的配置。 */
export async function updateAdminAnnouncementBanner(config: AnnouncementBannerConfig): Promise<AnnouncementBannerConfig> {
  const { data } = await apiClient.put<AnnouncementBannerConfig>('/admin/settings/announcement-banner', config)
  return data
}

/**
 * 按配置筛选并排序：去掉 hidden，order 里的按顺序在前，其余保持传入顺序。
 * 用户端和管理端预览共用，保证两边看到的顺序一致。
 */
export function applyAnnouncementBanner<T extends { id: number }>(items: T[], config: AnnouncementBannerConfig | null): T[] {
  if (!config) return items
  const hidden = new Set(config.hidden)
  const rank = new Map(config.order.map((id, i) => [id, i]))
  return items
    .filter((item) => !hidden.has(item.id))
    .map((item, i) => ({ item, i }))
    .sort((a, b) => (rank.get(a.item.id) ?? config.order.length + a.i) - (rank.get(b.item.id) ?? config.order.length + b.i))
    .map(({ item }) => item)
}
