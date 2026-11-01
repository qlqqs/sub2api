// CUSTOM: Partner (user side): invite card, profit share, region team spend
export default {
  partner: {
    title: 'Partner',
    tab: 'Profit Share',
    rate: 'Share {rate}',
    nextRate: '{rate} from {month}',
    basis: 'Paid from region profit, no agent commission',
    sinceAt: 'Since {time}',
    since: 'Since',
    lossCarried: 'Loss carried',
    agentCount: 'Channel managers',
    memberCount: 'Region members',
    currentMonth: 'This month (estimate) {month}',
    loadFailed: 'Failed to load profit share',
    deleted: 'Deleted',
    status: {
      current: 'Estimate',
      draft: 'Pending',
      confirmed: 'Confirmed',
      paid: 'Paid'
    },
    summary: {
      revenue: 'Revenue',
      accountCost: 'Upstream cost',
      commission: 'Commission',
      regionExpense: 'Region expenses',
      adjustment: 'Adjustments',
      profit: 'Profit',
      lossBefore: 'Loss carried in',
      lossAfter: 'Loss carried out',
      shareBase: 'Share base',
      rate: 'Rate',
      share: 'Share',
      shareEstimated: 'Estimated share',
      paidOn: 'Paid {date}'
    },
    lines: {
      title: 'By channel manager',
      line: 'Channel manager',
      members: 'Members',
      consumption: 'Spend',
      revenue: 'Revenue',
      account_cost: 'Upstream cost',
      commission: 'Commission',
      profit: 'Profit',
      empty: 'No data'
    },
    costs: {
      byGroup: 'Cost by group',
      byModel: 'Cost by model',
      group: 'Group',
      model: 'Model',
      empty: 'No data'
    },
    statements: {
      title: 'Statements',
      month: 'Month',
      status: 'Status',
      paidOn: 'Paid on',
      empty: 'No statements yet'
    },
    adjustments: {
      title: 'Adjustments',
      note: 'Note',
      amount: 'Amount',
      createdAt: 'Time',
      empty: 'No adjustments'
    },
    region: {
      title: 'Region team',
      directTeam: 'Partner direct team',
      lineAgent: 'Channel manager',
      noOwner: 'No owning agent',
      forbidden: 'Region team spend is only available to partners',
      notFound: 'Not in your region or not accessible',
      backToRegion: 'Back to region team',
      columns: {
        lineAgent: 'Channel manager',
        owner: 'Owning agent'
      },
      tabs: {
        customers: 'Region customers',
        lines: 'Channel managers'
      },
      empty: {
        noLines: 'No channel managers yet'
      }
    },
    agentView: {
      directCustomers: 'Direct customers',
      earnings: 'Agent earnings',
      lifetimeEarnings: 'Lifetime agent earnings {amount}',
      historicalHint: "No longer this channel manager's sub-agent; earlier customers still earn them the difference"
    }
  }
}
