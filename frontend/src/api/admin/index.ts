/**
 * Admin API barrel export
 * Centralized exports for all admin API modules
 */

import dashboardAPI from './dashboard'
import usersAPI from './users'
import groupsAPI from './groups'
import accountsAPI from './accounts'
import proxiesAPI from './proxies'
import redeemAPI from './redeem'
import promoAPI from './promo'
import announcementsAPI from './announcements'
import settingsAPI from './settings'
import systemAPI from './system'
import subscriptionsAPI from './subscriptions'
import usageAPI from './usage'
import geminiAPI from './gemini'
import antigravityAPI from './antigravity'
import grokAPI from './grok'
import cnProvidersAPI from './cnProviders'
import userAttributesAPI from './userAttributes'
import opsAPI from './ops'
import errorPassthroughAPI from './errorPassthrough'
import dataManagementAPI from './dataManagement'
import apiKeysAPI from './apiKeys'
import scheduledTestsAPI from './scheduledTests'
import backupAPI from './backup'
import tlsFingerprintProfileAPI from './tlsFingerprintProfile'
import channelsAPI from './channels'
import channelMonitorAPI from './channelMonitor'
import channelMonitorTemplateAPI from './channelMonitorTemplate'
import adminPaymentAPI from './payment'
import affiliatesAPI from './affiliates'
import adminAffiliateTeamAPI from './affiliateTeam'
import riskControlAPI from './riskControl'
import adminComplianceAPI from './compliance'
import auditAPI from './audit'
import pluginsAPI from './plugins'
// CUSTOM: 分组内单个模型倍率、美元汇率
import groupModelRatesAPI from './groupModelRates'
import fxRateAPI from './fxRate'
// CUSTOM: 财务明细页、账号模型成本倍率
import financeAPI from './finance'
import accountModelCostAPI from './accountModelCost'
// CUSTOM: QxAPI 客户端版本发布
import clientReleasesAPI from './clientReleases'

/**
 * Unified admin API object for convenient access
 */
export const adminAPI = {
  dashboard: dashboardAPI,
  users: usersAPI,
  groups: groupsAPI,
  accounts: accountsAPI,
  proxies: proxiesAPI,
  redeem: redeemAPI,
  promo: promoAPI,
  announcements: announcementsAPI,
  settings: settingsAPI,
  system: systemAPI,
  subscriptions: subscriptionsAPI,
  usage: usageAPI,
  gemini: geminiAPI,
  antigravity: antigravityAPI,
  grok: grokAPI,
  cnProviders: cnProvidersAPI,
  userAttributes: userAttributesAPI,
  ops: opsAPI,
  errorPassthrough: errorPassthroughAPI,
  dataManagement: dataManagementAPI,
  apiKeys: apiKeysAPI,
  scheduledTests: scheduledTestsAPI,
  backup: backupAPI,
  tlsFingerprintProfiles: tlsFingerprintProfileAPI,
  channels: channelsAPI,
  channelMonitor: channelMonitorAPI,
  channelMonitorTemplate: channelMonitorTemplateAPI,
  payment: adminPaymentAPI,
  affiliates: affiliatesAPI,
  affiliateTeam: adminAffiliateTeamAPI,
  riskControl: riskControlAPI,
  compliance: adminComplianceAPI,
  audit: auditAPI,
  plugins: pluginsAPI,
  groupModelRates: groupModelRatesAPI,
  fxRate: fxRateAPI,
  finance: financeAPI,
  accountModelCost: accountModelCostAPI,
  clientReleases: clientReleasesAPI
}

export {
  dashboardAPI,
  usersAPI,
  groupsAPI,
  accountsAPI,
  proxiesAPI,
  redeemAPI,
  promoAPI,
  announcementsAPI,
  settingsAPI,
  systemAPI,
  subscriptionsAPI,
  usageAPI,
  geminiAPI,
  antigravityAPI,
  grokAPI,
  cnProvidersAPI,
  userAttributesAPI,
  opsAPI,
  errorPassthroughAPI,
  dataManagementAPI,
  apiKeysAPI,
  scheduledTestsAPI,
  backupAPI,
  tlsFingerprintProfileAPI,
  channelsAPI,
  channelMonitorAPI,
  channelMonitorTemplateAPI,
  adminPaymentAPI,
  affiliatesAPI,
  adminAffiliateTeamAPI,
  riskControlAPI,
  adminComplianceAPI,
  auditAPI,
  pluginsAPI,
  groupModelRatesAPI,
  fxRateAPI,
  financeAPI,
  accountModelCostAPI,
  clientReleasesAPI
}

export default adminAPI

// Re-export types used by components
export type { AuditLog, AuditLogQuery, AuditLogListResponse } from './audit'
export type { BalanceHistoryItem } from './users'
export type { ErrorPassthroughRule, CreateRuleRequest, UpdateRuleRequest } from './errorPassthrough'
export type { BackupAgentHealth, DataManagementConfig } from './dataManagement'
export type { TLSFingerprintProfile, CreateProfileRequest, UpdateProfileRequest } from './tlsFingerprintProfile'
export type { ContentModerationConfig, ContentModerationLog, ModerationMode } from './riskControl'
export type {
  PluginInstallation,
  PluginCompatibility,
  PluginUISession,
  PluginTestResult
} from './plugins'
export type {
  ClientRelease,
  ClientReleaseFile,
  ClientReleaseStatus,
  ClientReleasePublishResult
} from './clientReleases'
