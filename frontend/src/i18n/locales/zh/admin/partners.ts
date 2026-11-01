// CUSTOM: 合伙人（管理端）
export default {
  partners: {
    description: '合伙人按区域利润分成；设立和挂靠都不能撤销',
    empty: '还没有合伙人',
    filterLabel: '所属合伙人',
    filterAll: '全部区域',
    levelFixed: '合伙人的档位固定为渠道经理',
    batchSkip: '已选中 {n} 个合伙人，档位固定，将跳过',
    autoAttached: '{agent} 已自动挂到 {partner}（{region}）名下',
    columns: {
      user: '账号',
      region: '所属合伙人',
      agentCount: '挂靠数',
      memberCount: '成员数',
      currentShare: '本月预估分成',
      lossCarried: '累计亏损',
      unconfirmedMonths: '待确认月数'
    },
    fields: {
      regionName: '区域名',
      shareRate: '分成比例'
    },
    errors: {
      rate: '比例须为 0–100，最多 2 位小数'
    },
    preview: {
      team: '归属客户 {owned} · 下级团队 {viaUpline}'
    },
    tabs: {
      overview: '概览',
      agents: '渠道经理',
      statements: '结算单'
    },
    create: {
      title: '设为合伙人',
      user: '用户',
      currentRegion: '此人目前在 {region}（{partner}）区域，设立后他本人和他以后的客户归新区域',
      submit: '设为合伙人',
      confirmTitle: '确认设为合伙人',
      confirmMessage: '把 {user} 设为「{region}」合伙人，分成比例 {rate}%。设为合伙人后不能撤销',
      success: '已设为合伙人'
    },
    attach: {
      title: '挂靠渠道经理',
      agent: '渠道经理',
      currentRegion: '此人目前在 {region}（{partner}）区域，挂靠后他和他的团队归本区域',
      submit: '挂靠',
      confirmTitle: '确认挂靠',
      confirmMessage: '把 {agent} 挂到「{region}」名下。挂靠后不能移出或改挂',
      success: '已挂靠'
    },
    edit: {
      title: '区域名与比例',
      rateHint: '当月比例 {rate}，新比例从下月起生效',
      savedRate: '已保存，新比例从下月起生效'
    },
    agents: {
      attachedAt: '挂靠时间',
      source: '来源',
      sourceManual: '手动',
      sourcePromote: '升级自动',
      team: '团队规模',
      empty: '还没有挂靠的渠道经理'
    },
    statement: {
      title: '{month} 结算单',
      adjustmentAmount: '金额（可为负）',
      adjustmentNote: '说明',
      addAdjustment: '添加调整项',
      paidOn: '打款日期',
      paidNote: '备注',
      markPaid: '标记已付',
      export: '导出 CSV',
      confirm: '确认',
      confirmTitle: '确认 {month} 结算单',
      confirmMessage: '确认后金额冻结，不能再加调整项',
      currentMonthHint: '当月不能确认',
      previousHint: '请先确认之前的月份',
      confirmed: '已确认',
      paid: '已标记为已付'
    }
  }
}
