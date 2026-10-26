// CUSTOM: 财务明细页、账号模型成本倍率、支付实例通道费率
export default {
  finance: {
    title: '财务明细',
    description: '充值、消耗、余额、分润、账号成本与其他支出，统计已确认收入与利润（人民币，北京时间）',
    toolbar: {
      range: '日期范围',
      granularity: '粒度',
      day: '按日',
      month: '按月',
      refresh: '刷新',
      export: '导出 CSV',
      exporting: '导出中…',
      exportFailed: '导出失败：{error}',
      rangeInvalid: '开始日期不能晚于结束日期',
      rangeTooLarge: '日期跨度不能超过 {max} 天',
      computedAt: '计算于 {time}'
    },
    warningsTitle: '注意',
    loadFailed: '加载失败：{error}',
    retry: '重试',
    noData: '暂无数据',
    total: '合计',
    tabs: {
      overview: '总览',
      profit: '利润明细',
      recharge: '充值与赠送',
      breakdown: '消耗与成本',
      affiliate: '分润',
      expenses: '其他支出',
      reconcile: '用户对账',
      settings: '设置'
    },
    overview: {
      periodTitle: '期间汇总',
      cards: {
        revenue: '已确认收入',
        revenueHint: '付费 {paid} · 分润转入 {commission}',
        accountCost: '账号成本',
        accountCostHint: '其中内部 / 测试消耗 {internal}',
        commission: '分润支出',
        expenses: '其他支出',
        expensesHint: '按摊销后计入',
        profit: '利润',
        profitHint: '收入 − 账号成本 − 分润 − 其他支出',
        profitMargin: '利润率',
        profitMarginHint: '利润 ÷ 已确认收入'
      },
      consumptionTitle: '余额消耗构成',
      consumption: '余额消耗合计',
      freeConsumed: '不计收入余额消耗',
      freeConsumedHint: '赠送 / 管理员加的余额被消耗，收入为 0，成本照记',
      overdraftConsumed: '透支消耗',
      snapshotTitle: '当前余额快照',
      snapshotHint: '当前时刻，不随日期范围变化。未消耗的余额是预收款，不计入利润',
      snapshot: {
        balanceTotal: '全站剩余余额',
        balancePaid: '付费余额',
        balancePaidCash: '对应实收 {amount}',
        balanceCommission: '分润转入余额',
        balanceFree: '不计收入余额',
        balanceUnattributed: '对账差额合计',
        balanceUnattributedHint: '实际余额 − 理论余额，非 0 时到「用户对账」查看',
        frozenBalance: '冻结余额',
        affiliateAvailable: '未提分润',
        affiliateFrozen: '冻结中分润'
      },
      trendTitle: '收入、成本与利润趋势',
      series: {
        revenue: '已确认收入',
        accountCost: '账号成本',
        commission: '分润',
        expenses: '其他支出',
        profit: '利润'
      }
    },
    profit: {
      columns: {
        period: '期间',
        revenue: '已确认收入',
        revenuePaid: '付费部分',
        revenueCommission: '分润转入部分',
        freeConsumed: '赠送消耗',
        overdraftConsumed: '透支消耗',
        consumption: '余额消耗',
        accountCost: '账号成本',
        accountCostInternal: '其中内部 / 测试',
        commission: '分润',
        expenses: '其他支出',
        profit: '利润',
        profitMargin: '利润率'
      }
    },
    recharge: {
      cards: {
        cashReceived: '实收现金',
        orderCount: '{count} 笔已入账订单',
        channelFee: '通道费',
        refundCash: '退款现金',
        netCash: '净实收',
        netCashHint: '实收 − 通道费 − 退款',
        principal: '到账本金',
        commissionTransferIn: '分润转入余额'
      },
      giftTitle: '赠送（不计收入入账）',
      gift: {
        recharge_bonus: '充值赠送',
        invitee_bonus: '首充奖励',
        promo: '优惠码',
        admin_adjust_in: '管理员加余额',
        admin_adjust_out: '管理员扣余额',
        redeem_code: '兑换码',
        total: '合计'
      },
      adminAdjustOutHint: '扣掉的金额，不计入赠送合计',
      ordersTitle: '订单明细',
      ordersTruncated: '只显示最近 500 笔，完整记录请到订单管理查看',
      viewInOrders: '在订单管理中查看该用户的订单',
      columns: {
        id: '订单',
        user: '用户',
        completedAt: '入账时间',
        paymentType: '支付方式',
        payAmount: '实付',
        channelFee: '通道费',
        principal: '本金',
        bonus: '赠送',
        refund: '退款（现金 / 扣回余额）',
        status: '状态'
      }
    },
    breakdown: {
      dims: {
        account: '按账号',
        group: '按分组',
        model: '按模型',
        user: '按用户'
      },
      columns: {
        name: '名称',
        costMode: '成本方式',
        requests: '请求数',
        images: '图片数',
        consumption: '余额消耗',
        subscriptionConsumption: '订阅消耗',
        accountCost: '账号成本',
        expenseCost: '关联支出',
        grossProfit: '毛利'
      },
      grossProfitHint: '毛利 = 余额消耗 − 账号成本，按消耗口径，仅供比较；利润以总览为准',
      grossProfitHintAccount: '毛利 = 余额消耗 − 账号成本 − 关联支出，按消耗口径，仅供比较；利润以总览为准',
      expenseCostHint: '期间内关联到该账号的其他支出（按摊销后金额），如固定成本账号的订阅费',
      subscriptionHint: '订阅计费的消耗仅供参考，不计入收入',
      costModes: {
        rate: '按倍率',
        model_rate: '模型倍率',
        per_image: '按张',
        fixed: '固定成本'
      },
      truncated: '只显示前 {limit} 行'
    },
    affiliate: {
      accruedTitle: '期间新产生分润（计为支出）',
      accrued: {
        agent_direct: '代理直推',
        agent_diff: '代理级差',
        invite_percent: '邀请返利（比例）',
        invite_fixed: '邀请返利（固定）',
        total: '合计'
      },
      flowTitle: '分润去向（不重复计支出）',
      transferred: '转入余额',
      withdrawn: '提现',
      snapshotTitle: '当前快照',
      available: '未提余额',
      frozen: '冻结中',
      topTitle: '期间分润最多的用户',
      columns: {
        user: '用户',
        accrued: '新产生',
        transferred: '转入余额',
        withdrawn: '提现',
        available: '未提余额'
      }
    },
    expenses: {
      add: '新增支出',
      createTitle: '新增支出',
      editTitle: '编辑支出',
      allCategories: '全部类别',
      rangeHint: '按发生日期筛选；摊销到范围外的部分也会列出',
      deleteConfirmTitle: '删除支出',
      deleteConfirm: '确定删除 {date} 的「{category}」支出 {amount} 吗？',
      created: '支出已添加',
      updated: '支出已更新',
      deleted: '支出已删除',
      categories: {
        upstream_subscription: '上游订阅账号',
        server: '服务器',
        domain: '域名',
        other: '其他'
      },
      columns: {
        occurredOn: '日期',
        category: '类别',
        amount: '金额',
        amortize: '摊销',
        account: '关联账号',
        note: '备注',
        createdBy: '录入人',
        actions: '操作'
      },
      amortizeDays: '{days} 天',
      amortizeOnce: '一次性',
      dailyAmount: '每日 {amount}',
      form: {
        occurredOn: '发生日期',
        category: '类别',
        amount: '金额（¥）',
        amortizeMode: '摊销方式',
        amortizeModes: { once: '一次性扣除', daily: '按天摊销' },
        amortizeDays: '摊销天数',
        amortizeOnceHint: '全额计入发生日期当天',
        amortizeDailyHint: '从发生日期起连续逐日平摊（含发生日），最后一天补齐尾差；订阅账号默认按 30 天摊销',
        account: '关联账号',
        accountNone: '不关联',
        note: '备注',
        notePlaceholder: '如 ChatGPT Pro #3'
      },
      errors: {
        occurredOnRequired: '请选择发生日期',
        amountInvalid: '金额须大于 0，最多 2 位小数',
        amortizeDaysInvalid: '按天摊销的天数须为 2–3660 的整数',
        noteTooLong: '备注不能超过 500 个字符'
      }
    },
    reconcile: {
      hint: '全时段对账，不受日期范围影响。差额 = 实际余额 − 理论余额，绝对值 ≥ 0.01 视为不平',
      onlyMismatch: '只看不平',
      searchPlaceholder: '搜索邮箱或用户 ID',
      mismatch: '不平',
      deletedUser: '已删除',
      columns: {
        user: '用户',
        paidIn: '付费本金',
        paidCash: '对应实收',
        freeIn: '不计收入入账',
        freeOut: '管理员扣回',
        commissionIn: '分润转入',
        refundDeducted: '退款扣回',
        consumption: '消耗',
        overdraft: '透支',
        expectedBalance: '理论余额',
        actualBalance: '实际余额',
        difference: '差额',
        remaining: '剩余（付费 / 分润 / 不计收入）'
      }
    },
    settings: {
      fixedCostTitle: '固定成本账号',
      fixedCostHint: '订阅号池等按月付费的账号：单次请求成本记 0，成本全部来自「其他支出」里录入的费用',
      searchPlaceholder: '搜索账号名称或 ID',
      selectedCount: '已选 {count} 个',
      deletedTag: '已删除',
      noAccounts: '没有账号',
      save: '保存设置',
      saved: '设置已保存'
    },
    accountModelCost: {
      title: '模型成本倍率',
      hint: '为个别模型单独设置成本倍率：按发往上游的模型匹配，命中时替换账号倍率（不是相乘），只影响账号成本统计，不影响用户计费。模型名支持末尾 * 通配，大小写不敏感；精确匹配优先，其次取前缀最长的通配规则。',
      add: '添加',
      model: '模型名',
      rate: '成本倍率',
      modelPlaceholder: '如 gpt-5.6-* 或 gpt-5.6-luna',
      ratePlaceholder: '倍率',
      remove: '删除',
      empty: '未设置，所有模型按账号倍率计算成本',
      retry: '重试',
      count: '{count} 个模型成本倍率',
      loadFailed: '模型成本倍率加载失败：{error}',
      saveFailed: '账号已保存，模型成本倍率保存失败：{error}',
      errors: {
        modelRequired: '请填写模型名',
        modelTooLong: '模型名不能超过 255 个字符',
        modelWhitespace: '模型名不能包含空格',
        wildcardPosition: '* 只能出现在末尾，且前面至少有一个字符',
        duplicate: '模型名重复（大小写不敏感）',
        rateRequired: '请填写倍率',
        rateInvalid: '倍率须为不小于 0 的数字',
        rateTooLarge: '倍率不能超过 {max}',
        rateDecimals: '倍率最多 4 位小数',
        tooMany: '每个账号最多 {max} 条模型成本倍率'
      }
    },
    channelFee: {
      label: '通道费率（%）',
      hint: '支付平台从每笔收款中抽取的比例，只用于财务统计；修改只影响之后的新订单',
      invalid: '通道费率须为 0–{max} 之间的数字，最多 2 位小数'
    }
  }
}
