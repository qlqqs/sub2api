package repository

// CUSTOM(fork): Shared by finance and balance-history totals; table alias must be o.
// Offline refund_amount is credited balance, so convert it to cash at the receipt
// ratio. Preserve the existing finance semantics for all online payment methods.
const paymentRefundCashSQL = `CASE WHEN o.status IN ('REFUNDED','PARTIALLY_REFUNDED') AND o.refund_at IS NOT NULL THEN
   CASE WHEN o.payment_type='offline' AND o.amount>0 THEN ROUND(COALESCE(o.refund_amount,0)*o.pay_amount/o.amount,2)
   ELSE COALESCE(o.refund_amount,0) END ELSE 0 END`
