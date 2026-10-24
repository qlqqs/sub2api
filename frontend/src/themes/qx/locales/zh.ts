// CUSTOM: QxAPI 主题文案（由 i18n/locales/zh/index.ts 引入）。
import nav from '../shell/locales/zh'
import dashboard from '../dashboard/locales/zh'
import studio from '../studio/locales/zh'
export default {
  qxTheme: {
    nav,
    dashboard,
    studio,
    listSeparator: '、',
    hero: {
      description: '注册后创建一个 API 密钥，在 {tools} 等工具里填入接口地址，就能调用模型。按实际用量从余额扣费。',
      register: '注册账号',
      models: '可用模型',
      protocols: '兼容协议',
      billing: '计费',
      billingValue: '按 tokens 用量'
    },
    console: {
      endpoint: '接口地址',
      copy: '复制',
      copied: '已复制',
      keyHint: '把 $API_KEY 换成你在控制台创建的密钥',
      prompt: '帮我列一份实验报告提纲'
    },
    pricing: {
      kicker: 'PRICING',
      title: '模型价格',
      lead: '与模型广场一致，按每百万 tokens 计，从余额扣除。',
      model: '模型',
      context: '上下文',
      input: '输入',
      output: '输出',
      cacheRead: '缓存读取',
      perImage: '/ 张',
      perRequest: '/ 次',
      timePricingNote: '部分模型在特定时段价格不同，详见模型广场。',
      more: '在模型广场查看缓存写入等完整价格'
    },
    steps: {
      kicker: 'QUICK START',
      title: '三步开始使用',
      register: {
        title: '注册并充值',
        description: '邮箱注册，在控制台充值余额'
      },
      key: {
        title: '创建 API 密钥',
        description: '在「API 密钥」页面新建一个密钥'
      },
      ccs: {
        label: '一键导入',
        badge: '推荐',
        title: 'CC Switch',
        description: '在「API 密钥」页点密钥旁的「一键导入」，选择 CC Switch 或 Magpie，自动填好接口地址和密钥',
        keys: '前往 API 密钥',
        download: '下载 CC Switch'
      },
      manual: {
        label: '手动配置',
        title: 'Base URL',
        description: '在工具里填入对应地址和密钥'
      },
      paths: '支持接口'
    }
  }
}
