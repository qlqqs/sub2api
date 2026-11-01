// CUSTOM: 合伙人（用户端）：邀请页卡片、利润分成、区域团队消费
export default {
  partner: {
    title: '合伙人',
    tab: '利润分成',
    rate: '分成比例 {rate}',
    nextRate: '{month} 起 {rate}',
    basis: '按区域利润分成，不拿代理分润',
    sinceAt: '{time} 设立',
    since: '设立时间',
    lossCarried: '累计亏损',
    agentCount: '渠道经理',
    memberCount: '区域成员',
    currentMonth: '本月预估 {month}',
    loadFailed: '加载利润分成失败',
    deleted: '已删除',
    status: {
      current: '预估',
      draft: '待确认',
      confirmed: '已确认',
      paid: '已付'
    },
    summary: {
      revenue: '收入',
      accountCost: '上游成本',
      commission: '分润',
      regionExpense: '专项支出',
      adjustment: '调整项',
      profit: '利润',
      lossBefore: '期初累计亏损',
      lossAfter: '期末累计亏损',
      shareBase: '分成基数',
      rate: '比例',
      share: '分成',
      shareEstimated: '预计分成',
      paidOn: '{date} 已付'
    },
    lines: {
      title: '按渠道经理拆分',
      line: '渠道经理',
      members: '成员',
      consumption: '消费',
      revenue: '收入',
      account_cost: '上游成本',
      commission: '分润',
      profit: '利润',
      empty: '暂无数据'
    },
    costs: {
      byGroup: '成本 · 按分组',
      byModel: '成本 · 按模型',
      group: '分组',
      model: '模型',
      empty: '暂无数据'
    },
    statements: {
      title: '结算单',
      month: '月份',
      status: '状态',
      paidOn: '打款日期',
      empty: '暂无结算单'
    },
    adjustments: {
      title: '调整项',
      note: '说明',
      amount: '金额',
      createdAt: '时间',
      empty: '没有调整项'
    },
    region: {
      title: '区域团队',
      directTeam: '合伙人直属团队',
      lineAgent: '渠道经理',
      noOwner: '无归属代理',
      forbidden: '区域团队消费仅对合伙人开放',
      notFound: '不在你的区域内或无权查看',
      backToRegion: '返回区域团队',
      columns: {
        lineAgent: '所属渠道经理',
        owner: '归属代理'
      },
      tabs: {
        customers: '区域客户',
        lines: '渠道经理'
      },
      empty: {
        noLines: '还没有渠道经理'
      }
    },
    agentView: {
      directCustomers: '直属客户',
      earnings: '代理收益',
      lifetimeEarnings: '累计代理收益 {amount}',
      historicalHint: '已不是该渠道经理的分销代理，之前的客户仍为该渠道经理带来差额收益'
    }
  }
}
