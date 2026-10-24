// CUSTOM: 创作工作台页面占满外壳内容区（QxShell 去掉内边距），见 StudioView.vue。
export {}

declare module 'vue-router' {
  interface RouteMeta {
    qxFullBleed?: boolean
  }
}
