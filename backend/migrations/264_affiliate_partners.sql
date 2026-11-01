-- CUSTOM(fork): 合伙人区域、按月利润分成及冻结结算单。
CREATE TABLE IF NOT EXISTS affiliate_partners (
    user_id     BIGINT PRIMARY KEY,
    region_name VARCHAR(64) NOT NULL,
    created_by  BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),   -- 设立时刻 = 合伙人自己团队的计入起点
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 分成比例按月生效：某月比例 = effective_month <= 该月的最新一行
CREATE TABLE IF NOT EXISTS affiliate_partner_rates (
    partner_user_id BIGINT NOT NULL,
    effective_month DATE NOT NULL,                     -- 北京时间自然月 1 日
    share_rate      NUMERIC(6,4) NOT NULL CHECK (share_rate >= 0 AND share_rate <= 1),
    created_by      BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (partner_user_id, effective_month)
);

-- 挂靠：主键保证一个渠道经理永远只属于一个合伙人；不提供删除接口
CREATE TABLE IF NOT EXISTS affiliate_partner_agents (
    agent_user_id   BIGINT PRIMARY KEY,
    partner_user_id BIGINT NOT NULL,
    attached_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- 该渠道经理团队的计入起点
    attached_by     BIGINT NULL,                        -- 自动升级触发的自动挂靠为 NULL
    source          VARCHAR(16) NOT NULL                -- manual / promote
);
CREATE INDEX IF NOT EXISTS idx_affiliate_partner_agents_partner ON affiliate_partner_agents(partner_user_id);

CREATE TABLE IF NOT EXISTS affiliate_partner_adjustments (
    id              BIGSERIAL PRIMARY KEY,
    partner_user_id BIGINT NOT NULL,
    month           DATE NOT NULL,
    amount          NUMERIC(20,2) NOT NULL CHECK (amount <> 0),
    note            VARCHAR(500) NOT NULL,
    created_by      BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_affiliate_partner_adjustments_month ON affiliate_partner_adjustments(partner_user_id, month);

CREATE TABLE IF NOT EXISTS affiliate_partner_statements (
    id              BIGSERIAL PRIMARY KEY,
    partner_user_id BIGINT NOT NULL,
    month           DATE NOT NULL,
    status          VARCHAR(16) NOT NULL,              -- confirmed / paid
    share_rate      NUMERIC(6,4) NOT NULL,
    consumption     NUMERIC(20,8) NOT NULL,
    revenue         NUMERIC(20,8) NOT NULL,
    account_cost    NUMERIC(20,8) NOT NULL,
    commission      NUMERIC(20,8) NOT NULL,
    region_expense  NUMERIC(20,8) NOT NULL,
    adjustment      NUMERIC(20,8) NOT NULL,
    profit          NUMERIC(20,8) NOT NULL,
    loss_before     NUMERIC(20,8) NOT NULL,
    loss_after      NUMERIC(20,8) NOT NULL,
    share_base      NUMERIC(20,8) NOT NULL,
    share_amount    NUMERIC(20,2) NOT NULL,
    detail          JSONB NOT NULL,                    -- lines / cost_by_group / cost_by_model / adjustments / warnings
    confirmed_by    BIGINT NOT NULL,
    confirmed_at    TIMESTAMPTZ NOT NULL,
    paid_by         BIGINT NULL,
    paid_on         DATE NULL,
    paid_note       VARCHAR(500) NULL,
    UNIQUE (partner_user_id, month)
);

ALTER TABLE finance_expenses ADD COLUMN IF NOT EXISTS partner_user_id BIGINT NULL; -- 区域专项支出

COMMENT ON TABLE affiliate_partners IS '合伙人区域';
COMMENT ON COLUMN affiliate_partners.user_id IS '合伙人用户ID';
COMMENT ON COLUMN affiliate_partners.region_name IS '区域名称';
COMMENT ON COLUMN affiliate_partners.created_by IS '操作管理员ID';
COMMENT ON COLUMN affiliate_partners.created_at IS '创建及生效时间';
COMMENT ON COLUMN affiliate_partners.updated_at IS '更新时间';
COMMENT ON TABLE affiliate_partner_rates IS '合伙人按月分成比例';
COMMENT ON COLUMN affiliate_partner_rates.partner_user_id IS '合伙人用户ID';
COMMENT ON COLUMN affiliate_partner_rates.effective_month IS '北京时间生效月份首日';
COMMENT ON COLUMN affiliate_partner_rates.share_rate IS '分成比例，0到1';
COMMENT ON COLUMN affiliate_partner_rates.created_by IS '操作管理员ID';
COMMENT ON COLUMN affiliate_partner_rates.created_at IS '创建及生效时间';
COMMENT ON TABLE affiliate_partner_agents IS '渠道经理永久挂靠';
COMMENT ON COLUMN affiliate_partner_agents.agent_user_id IS '渠道经理用户ID';
COMMENT ON COLUMN affiliate_partner_agents.partner_user_id IS '合伙人用户ID';
COMMENT ON COLUMN affiliate_partner_agents.attached_at IS '永久挂靠生效时间';
COMMENT ON COLUMN affiliate_partner_agents.attached_by IS '操作管理员ID，自动升级为空';
COMMENT ON COLUMN affiliate_partner_agents.source IS 'manual手动挂靠或promote升级挂靠';
COMMENT ON TABLE affiliate_partner_adjustments IS '合伙人月度利润调整项';
COMMENT ON COLUMN affiliate_partner_adjustments.id IS '记录ID';
COMMENT ON COLUMN affiliate_partner_adjustments.partner_user_id IS '合伙人用户ID';
COMMENT ON COLUMN affiliate_partner_adjustments.month IS '北京时间归属月份首日';
COMMENT ON COLUMN affiliate_partner_adjustments.amount IS '调整金额';
COMMENT ON COLUMN affiliate_partner_adjustments.note IS '调整说明';
COMMENT ON COLUMN affiliate_partner_adjustments.created_by IS '操作管理员ID';
COMMENT ON COLUMN affiliate_partner_adjustments.created_at IS '创建及生效时间';
COMMENT ON TABLE affiliate_partner_statements IS '合伙人已确认结算单';
COMMENT ON COLUMN affiliate_partner_statements.id IS '记录ID';
COMMENT ON COLUMN affiliate_partner_statements.partner_user_id IS '合伙人用户ID';
COMMENT ON COLUMN affiliate_partner_statements.month IS '北京时间归属月份首日';
COMMENT ON COLUMN affiliate_partner_statements.status IS 'confirmed已确认或paid已付';
COMMENT ON COLUMN affiliate_partner_statements.share_rate IS '分成比例，0到1';
COMMENT ON COLUMN affiliate_partner_statements.consumption IS '区域余额消耗';
COMMENT ON COLUMN affiliate_partner_statements.revenue IS '区域确认收入';
COMMENT ON COLUMN affiliate_partner_statements.account_cost IS '区域上游请求成本';
COMMENT ON COLUMN affiliate_partner_statements.commission IS '区域分润流水金额';
COMMENT ON COLUMN affiliate_partner_statements.region_expense IS '区域专项支出';
COMMENT ON COLUMN affiliate_partner_statements.adjustment IS '调整项合计';
COMMENT ON COLUMN affiliate_partner_statements.profit IS '区域利润';
COMMENT ON COLUMN affiliate_partner_statements.loss_before IS '期初累计亏损';
COMMENT ON COLUMN affiliate_partner_statements.loss_after IS '期末累计亏损';
COMMENT ON COLUMN affiliate_partner_statements.share_base IS '抵扣亏损后分成基数';
COMMENT ON COLUMN affiliate_partner_statements.share_amount IS '合伙人分成';
COMMENT ON COLUMN affiliate_partner_statements.detail IS '确认时完整明细快照';
COMMENT ON COLUMN affiliate_partner_statements.confirmed_by IS '确认管理员ID';
COMMENT ON COLUMN affiliate_partner_statements.confirmed_at IS '确认时间';
COMMENT ON COLUMN affiliate_partner_statements.paid_by IS '付款登记管理员ID';
COMMENT ON COLUMN affiliate_partner_statements.paid_on IS '北京时间付款日期';
COMMENT ON COLUMN affiliate_partner_statements.paid_note IS '付款备注';
COMMENT ON COLUMN finance_expenses.partner_user_id IS '区域专项支出的合伙人用户ID，空为公共支出';
