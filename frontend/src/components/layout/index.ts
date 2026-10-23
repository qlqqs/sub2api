/**
 * Layout Components
 * Export all layout components for easy importing
 */

export { default as AppLayout } from './AppLayout.vue'
export { default as AppSidebar } from './AppSidebar.vue'
export { default as AppHeader } from './AppHeader.vue'
// CUSTOM: QxAPI 主题外壳，主题关闭时内部渲染上游 AuthLayout.vue（见 themes/qx/config.ts）
export { default as AuthLayout } from '@/themes/qx/QxAuthLayout.vue'
