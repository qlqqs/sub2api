/** Channel status values (must match service.Status* constants in Go). */
export const CHANNEL_STATUS_ACTIVE = 'active' as const
export const CHANNEL_STATUS_DISABLED = 'disabled' as const
export type ChannelStatus = typeof CHANNEL_STATUS_ACTIVE | typeof CHANNEL_STATUS_DISABLED

/** Billing mode values (must match service.BillingMode* constants in Go). */
export const BILLING_MODE_TOKEN = 'token' as const
export const BILLING_MODE_PER_REQUEST = 'per_request' as const
export const BILLING_MODE_IMAGE = 'image' as const
export const BILLING_MODE_VIDEO = 'video' as const
export type BillingMode =
  | typeof BILLING_MODE_TOKEN
  | typeof BILLING_MODE_PER_REQUEST
  | typeof BILLING_MODE_IMAGE
  | typeof BILLING_MODE_VIDEO

/** Supported request reasoning effort levels, ordered from lowest to highest. */
export const REASONING_EFFORT_LEVELS = ['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'] as const
export type ReasoningEffortLevel = typeof REASONING_EFFORT_LEVELS[number]

/** Billing-model-source values (must match service.BillingModelSource* constants in Go). */
export const BILLING_MODEL_SOURCE_REQUESTED = 'requested' as const
export const BILLING_MODEL_SOURCE_UPSTREAM = 'upstream' as const
export const BILLING_MODEL_SOURCE_CHANNEL_MAPPED = 'channel_mapped' as const
export const BILLING_MODEL_SOURCE_RESPONSE = 'response_model' as const
export type BillingModelSource =
  | typeof BILLING_MODEL_SOURCE_REQUESTED
  | typeof BILLING_MODEL_SOURCE_UPSTREAM
  | typeof BILLING_MODEL_SOURCE_CHANNEL_MAPPED
  | typeof BILLING_MODEL_SOURCE_RESPONSE

/**
 * Video billing tiers (must match the backend video billing resolutions).
 * The channel base price is the 480p tier; `<resolution>-ref` is the tier used
 * when the request carries a reference video.
 */
export const VIDEO_RESOLUTIONS = ['480p', '720p', '1080p', '4k'] as const
export const VIDEO_BASE_RESOLUTION = '480p'
export const VIDEO_REF_TIER_SUFFIX = '-ref'
export const VIDEO_TIER_LABELS = [
  ...VIDEO_RESOLUTIONS,
  ...VIDEO_RESOLUTIONS.map((r) => `${r}${VIDEO_REF_TIER_SUFFIX}`)
]
