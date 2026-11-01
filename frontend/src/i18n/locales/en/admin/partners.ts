// CUSTOM: Partners (admin)
export default {
  partners: {
    description: 'Partners share region profit; creating and attaching cannot be undone',
    empty: 'No partners yet',
    filterLabel: 'Partner',
    filterAll: 'All regions',
    levelFixed: 'A partner is fixed at channel manager level',
    batchSkip: '{n} selected partner(s) keep their level and will be skipped',
    autoAttached: '{agent} was attached to {partner} ({region})',
    columns: {
      region: 'Partner',
      agentCount: 'Attached',
      memberCount: 'Members',
      currentShare: 'Est. share this month',
      lossCarried: 'Loss carried',
      unconfirmedMonths: 'Unconfirmed months'
    },
    fields: {
      regionName: 'Region name',
      shareRate: 'Share rate'
    },
    errors: {
      rate: 'Rate must be 0–100 with up to 2 decimals'
    },
    preview: {
      team: 'Customers {owned} · Downline {viaUpline}'
    },
    tabs: {
      overview: 'Overview',
      agents: 'Channel managers',
      statements: 'Statements'
    },
    create: {
      title: 'Add partner',
      user: 'User',
      currentRegion: 'Currently in region {region} ({partner}); they and their future customers move to the new region',
      submit: 'Make partner',
      confirmTitle: 'Confirm partner',
      confirmMessage: 'Make {user} the partner of "{region}" at {rate}%. This cannot be undone',
      success: 'Partner created'
    },
    attach: {
      title: 'Attach channel manager',
      agent: 'Channel manager',
      currentRegion: 'Currently in region {region} ({partner}); they and their team move to this region',
      submit: 'Attach',
      confirmTitle: 'Confirm attach',
      confirmMessage: 'Attach {agent} to "{region}". This cannot be removed or moved later',
      success: 'Attached'
    },
    edit: {
      title: 'Region name and rate',
      rateHint: 'This month {rate}; a new rate takes effect next month',
      savedRate: 'Saved; the new rate takes effect next month'
    },
    agents: {
      attachedAt: 'Attached at',
      source: 'Source',
      sourceManual: 'Manual',
      sourcePromote: 'On promotion',
      team: 'Team',
      empty: 'No attached channel managers'
    },
    statement: {
      title: 'Statement {month}',
      adjustmentAmount: 'Amount (may be negative)',
      adjustmentNote: 'Note',
      addAdjustment: 'Add adjustment',
      paidOn: 'Paid on',
      paidNote: 'Note',
      markPaid: 'Mark paid',
      export: 'Export CSV',
      confirm: 'Confirm',
      confirmTitle: 'Confirm statement {month}',
      confirmMessage: 'Amounts are frozen after confirming and no more adjustments can be added',
      currentMonthHint: 'The current month cannot be confirmed',
      previousHint: 'Confirm earlier months first',
      confirmed: 'Confirmed',
      paid: 'Marked as paid'
    }
  }
}
