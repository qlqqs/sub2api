-- 253: 分组内单个模型倍率（fork 自有表，不进 Ent schema）。
--
-- 一行一条规则：model_pattern 存归一化后的模型名（小写；claude- 前缀的 . 换成 -），
-- 可带末尾 * 通配。计费时按计费模型匹配：精确优先，其次前缀最长的通配；命中后
-- 用户专属倍率不再作用于该模型。读路径是进程内整表快照（30 秒过期）。
--
-- 分组是软删除：已删除分组的规则留在表里无害（读快照时 JOIN groups 过滤），
-- 分组硬删除时级联删除。只新增表，可重复执行。
CREATE TABLE IF NOT EXISTS group_model_rate_multipliers (
    group_id        BIGINT        NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    model_pattern   VARCHAR(255)  NOT NULL,
    rate_multiplier DECIMAL(10,4) NOT NULL CHECK (rate_multiplier > 0),
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, model_pattern)
);

COMMENT ON TABLE group_model_rate_multipliers IS '分组内单个模型倍率（覆盖分组倍率与用户专属倍率）';
COMMENT ON COLUMN group_model_rate_multipliers.model_pattern IS '归一化模型名，可带末尾 * 通配';
