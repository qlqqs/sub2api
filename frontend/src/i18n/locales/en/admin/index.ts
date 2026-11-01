import overview from './overview'
import channels from './channels'
import accounts from './accounts'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import audit from './audit'
import promptAudit from './promptAudit'
import plugins from './plugins'
import finance from './finance' // CUSTOM
import financeRecalculation from './financeRecalculation' // CUSTOM
import clientReleases from './clientReleases' // CUSTOM
import partners from './partners' // CUSTOM: 合伙人

export default {
  ...overview,
  ...channels,
  ...accounts,
  ...resources,
  ...ops,
  ...settings,
  ...audit,
  ...promptAudit,
  ...plugins,
  ...finance,
  ...financeRecalculation,
  ...clientReleases,
  ...partners,
}
