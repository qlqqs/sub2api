// CUSTOM: finance ledger page, account model cost multipliers, payment channel fee rate
export default {
  finance: {
    title: 'Finance Ledger',
    description: 'Recharges, consumption, balances, commissions, account costs and other expenses, with recognized revenue and profit (CNY, Beijing time)',
    toolbar: {
      range: 'Date range',
      granularity: 'Granularity',
      day: 'Daily',
      month: 'Monthly',
      refresh: 'Refresh',
      export: 'Export CSV',
      exporting: 'Exporting…',
      exportFailed: 'Export failed: {error}',
      rangeInvalid: 'Start date must not be after end date',
      rangeTooLarge: 'Date range cannot exceed {max} days',
      computedAt: 'Computed at {time}'
    },
    warningsTitle: 'Note',
    loadFailed: 'Failed to load: {error}',
    retry: 'Retry',
    noData: 'No data',
    total: 'Total',
    tabs: {
      overview: 'Overview',
      profit: 'Profit details',
      recharge: 'Recharges & gifts',
      breakdown: 'Consumption & cost',
      affiliate: 'Commissions',
      expenses: 'Other expenses',
      reconcile: 'User reconciliation',
      settings: 'Settings'
    },
    overview: {
      periodTitle: 'Period summary',
      cards: {
        revenue: 'Recognized revenue',
        revenueHint: 'Paid {paid} · Commission transfers {commission}',
        accountCost: 'Account cost',
        accountCostHint: 'Internal / test consumption {internal}',
        commission: 'Commission expense',
        expenses: 'Other expenses',
        expensesHint: 'After amortization',
        profit: 'Profit',
        profitHint: 'Revenue − account cost − commission − other expenses',
        profitMargin: 'Profit margin',
        profitMarginHint: 'Profit ÷ recognized revenue'
      },
      consumptionTitle: 'Balance consumption',
      consumption: 'Total balance consumed',
      freeConsumed: 'Non-revenue balance consumed',
      freeConsumedHint: 'Gifted / admin-added balance consumed: no revenue, cost still counted',
      overdraftConsumed: 'Overdraft consumed',
      snapshotTitle: 'Current balance snapshot',
      snapshotHint: 'Current moment, independent of the date range. Unconsumed balance is prepaid (a liability) and not counted as profit',
      snapshot: {
        balanceTotal: 'Total remaining balance',
        balancePaid: 'Paid balance',
        balancePaidCash: 'Cash value {amount}',
        balanceCommission: 'Commission-transfer balance',
        balanceFree: 'Non-revenue balance',
        balanceUnattributed: 'Reconciliation difference',
        balanceUnattributedHint: 'Actual − expected balance; see User reconciliation when non-zero',
        frozenBalance: 'Frozen balance',
        affiliateAvailable: 'Unwithdrawn commission',
        affiliateFrozen: 'Frozen commission'
      },
      trendTitle: 'Revenue, cost and profit trend',
      series: {
        revenue: 'Recognized revenue',
        accountCost: 'Account cost',
        commission: 'Commission',
        expenses: 'Other expenses',
        profit: 'Profit'
      }
    },
    profit: {
      columns: {
        period: 'Period',
        revenue: 'Recognized revenue',
        revenuePaid: 'From paid',
        revenueCommission: 'From commission transfers',
        freeConsumed: 'Gift consumed',
        overdraftConsumed: 'Overdraft',
        consumption: 'Balance consumed',
        accountCost: 'Account cost',
        accountCostInternal: 'Internal / test',
        commission: 'Commission',
        expenses: 'Other expenses',
        profit: 'Profit',
        profitMargin: 'Margin'
      }
    },
    recharge: {
      cards: {
        cashReceived: 'Cash received',
        orderCount: '{count} credited orders',
        channelFee: 'Channel fees',
        refundCash: 'Cash refunded',
        netCash: 'Net cash',
        netCashHint: 'Received − channel fees − refunds',
        principal: 'Credited principal',
        commissionTransferIn: 'Commission transferred to balance'
      },
      giftTitle: 'Gifts (non-revenue credits)',
      gift: {
        recharge_bonus: 'Recharge bonus',
        invitee_bonus: 'First-recharge reward',
        promo: 'Promo codes',
        admin_adjust_in: 'Admin added',
        admin_adjust_out: 'Admin deducted',
        redeem_code: 'Redeem codes',
        total: 'Total'
      },
      adminAdjustOutHint: 'Amount deducted; not included in the gift total',
      ordersTitle: 'Orders',
      ordersTruncated: 'Showing the latest 500 orders; see Order Management for the full list',
      viewInOrders: "View this user's orders in Order Management",
      columns: {
        id: 'Order',
        user: 'User',
        completedAt: 'Credited at',
        paymentType: 'Method',
        payAmount: 'Paid',
        channelFee: 'Channel fee',
        principal: 'Principal',
        bonus: 'Bonus',
        refund: 'Refund (cash / balance deducted)',
        status: 'Status'
      }
    },
    breakdown: {
      dims: {
        account: 'By account',
        group: 'By group',
        model: 'By model',
        user: 'By user'
      },
      columns: {
        name: 'Name',
        costMode: 'Cost mode',
        requests: 'Requests',
        images: 'Images',
        consumption: 'Balance consumed',
        subscriptionConsumption: 'Subscription consumed',
        accountCost: 'Account cost',
        expenseCost: 'Linked expenses',
        grossProfit: 'Gross profit'
      },
      grossProfitHint: 'Gross profit = balance consumed − account cost (consumption basis, for comparison only); see Overview for profit',
      grossProfitHintAccount: 'Gross profit = balance consumed − account cost − linked expenses (consumption basis, for comparison only); see Overview for profit',
      expenseCostHint: 'Other expenses linked to this account within the range (after amortization), e.g. subscription fees of a fixed-cost account',
      subscriptionHint: 'Subscription consumption is for reference and not counted as revenue',
      costModes: {
        rate: 'Multiplier',
        model_rate: 'Model multiplier',
        per_image: 'Per image',
        fixed: 'Fixed cost'
      },
      truncated: 'Showing the first {limit} rows'
    },
    affiliate: {
      accruedTitle: 'Commission accrued in period (expense)',
      accrued: {
        agent_direct: 'Agent direct',
        agent_diff: 'Agent differential',
        invite_percent: 'Invite rebate (percent)',
        invite_fixed: 'Invite rebate (fixed)',
        total: 'Total'
      },
      flowTitle: 'Where commission went (not counted twice)',
      transferred: 'Transferred to balance',
      withdrawn: 'Withdrawn',
      snapshotTitle: 'Current snapshot',
      available: 'Unwithdrawn',
      frozen: 'Frozen',
      topTitle: 'Top beneficiaries in period',
      columns: {
        user: 'User',
        accrued: 'Accrued',
        transferred: 'Transferred',
        withdrawn: 'Withdrawn',
        available: 'Unwithdrawn'
      }
    },
    expenses: {
      add: 'Add expense',
      createTitle: 'Add expense',
      editTitle: 'Edit expense',
      allCategories: 'All categories',
      rangeHint: 'Filtered by date incurred; amortization outside the range is still listed',
      deleteConfirmTitle: 'Delete expense',
      deleteConfirm: 'Delete the "{category}" expense of {amount} on {date}?',
      created: 'Expense added',
      updated: 'Expense updated',
      deleted: 'Expense deleted',
      categories: {
        upstream_subscription: 'Upstream subscription account',
        server: 'Server',
        domain: 'Domain',
        other: 'Other'
      },
      columns: {
        occurredOn: 'Date',
        category: 'Category',
        amount: 'Amount',
        amortize: 'Amortization',
        account: 'Account',
        note: 'Note',
        createdBy: 'Entered by',
        actions: 'Actions'
      },
      amortizeDays: '{days} days',
      amortizeOnce: 'One-off',
      dailyAmount: '{amount} / day',
      form: {
        occurredOn: 'Date incurred',
        category: 'Category',
        amount: 'Amount (¥)',
        amortizeMode: 'Amortization',
        amortizeModes: { once: 'Deduct at once', daily: 'Spread per day' },
        amortizeDays: 'Amortization days',
        amortizeOnceHint: 'The full amount is booked on the date incurred',
        amortizeDailyHint: 'Spread evenly per day starting on the date incurred; the last day absorbs rounding. Subscription accounts default to 30 days',
        account: 'Linked account',
        accountNone: 'None',
        note: 'Note',
        notePlaceholder: 'e.g. ChatGPT Pro #3'
      },
      errors: {
        occurredOnRequired: 'Please choose the date incurred',
        amountInvalid: 'Amount must be greater than 0 with at most 2 decimals',
        amortizeDaysInvalid: 'Days must be an integer from 2 to 3660',
        noteTooLong: 'Note cannot exceed 500 characters'
      }
    },
    reconcile: {
      hint: 'All-time reconciliation, independent of the date range. Difference = actual − expected balance; |difference| ≥ 0.01 is a mismatch',
      onlyMismatch: 'Mismatches only',
      searchPlaceholder: 'Search email or user ID',
      mismatch: 'Mismatch',
      deletedUser: 'Deleted',
      columns: {
        user: 'User',
        paidIn: 'Paid principal',
        paidCash: 'Cash value',
        freeIn: 'Non-revenue credits',
        freeOut: 'Admin deducted',
        commissionIn: 'Commission in',
        refundDeducted: 'Refund deducted',
        consumption: 'Consumed',
        overdraft: 'Overdraft',
        expectedBalance: 'Expected balance',
        actualBalance: 'Actual balance',
        difference: 'Difference',
        remaining: 'Remaining (paid / commission / non-revenue)'
      }
    },
    settings: {
      fixedCostTitle: 'Fixed-cost accounts',
      fixedCostHint: 'Accounts paid by subscription (e.g. pooled plans): per-request cost is 0, all cost comes from expenses entered under Other expenses',
      searchPlaceholder: 'Search account name or ID',
      selectedCount: '{count} selected',
      deletedTag: 'Deleted',
      noAccounts: 'No accounts',
      save: 'Save settings',
      saved: 'Settings saved'
    },
    accountModelCost: {
      title: 'Model cost multipliers',
      hint: 'Set a separate cost multiplier for specific models. Matched against the model sent upstream; a match replaces the account multiplier (not multiplied). Only affects account cost statistics, not user billing. Model names support a trailing * wildcard and are case-insensitive; exact matches win, then the longest wildcard prefix.',
      add: 'Add',
      model: 'Model',
      rate: 'Cost multiplier',
      modelPlaceholder: 'e.g. gpt-5.6-* or gpt-5.6-luna',
      ratePlaceholder: 'Multiplier',
      remove: 'Remove',
      empty: 'Not set; all models use the account multiplier for cost',
      retry: 'Retry',
      count: '{count} model cost multipliers',
      loadFailed: 'Failed to load model cost multipliers: {error}',
      saveFailed: 'Account saved, but model cost multipliers failed to save: {error}',
      errors: {
        modelRequired: 'Model name is required',
        modelTooLong: 'Model name cannot exceed 255 characters',
        modelWhitespace: 'Model name cannot contain spaces',
        wildcardPosition: '* may only appear at the end, after at least one character',
        duplicate: 'Duplicate model name (case-insensitive)',
        rateRequired: 'Multiplier is required',
        rateInvalid: 'Multiplier must be a number ≥ 0',
        rateTooLarge: 'Multiplier cannot exceed {max}',
        rateDecimals: 'Multiplier allows at most 4 decimals',
        tooMany: 'At most {max} model cost multipliers per account'
      }
    },
    channelFee: {
      label: 'Channel fee rate (%)',
      hint: 'Share of each payment taken by the payment platform. Used only for finance statistics; changes apply to new orders only',
      invalid: 'Channel fee rate must be between 0 and {max} with at most 2 decimals'
    }
  }
}
