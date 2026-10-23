-- 代理团队消费页：按用户 × 北京时间自然日 × 模型 × 分组汇总余额消费。
-- 由 usage_logs 上的语句级触发器增量维护（实时、无水位、无定时任务）；
-- 迁移内先建触发器再回填，同一事务内完成，回填与触发器之间没有空档。

CREATE TABLE IF NOT EXISTS usage_user_daily_rollups (
    user_id BIGINT NOT NULL,
    bucket_date DATE NOT NULL,
    model VARCHAR(100) NOT NULL,
    group_id BIGINT NOT NULL DEFAULT 0,
    requests BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_creation_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    image_count BIGINT NOT NULL DEFAULT 0,
    video_count BIGINT NOT NULL DEFAULT 0,
    actual_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    last_used_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, bucket_date, model, group_id)
);

CREATE INDEX IF NOT EXISTS idx_usage_user_daily_rollups_bucket_date
    ON usage_user_daily_rollups (bucket_date);

COMMENT ON TABLE usage_user_daily_rollups IS '按用户、北京时间自然日、请求模型、分组汇总的余额消费（billing_type = 0），由 usage_logs 触发器维护。';
COMMENT ON COLUMN usage_user_daily_rollups.bucket_date IS '北京时间自然日。';
COMMENT ON COLUMN usage_user_daily_rollups.model IS '用户请求的模型：COALESCE(NULLIF(BTRIM(requested_model), ''''), model)。';
COMMENT ON COLUMN usage_user_daily_rollups.group_id IS '分组 ID，0 表示无分组。';
COMMENT ON COLUMN usage_user_daily_rollups.last_used_at IS '该桶内最近一次消费时间；删除明细时不回退。';

-- INSERT / DELETE / UPDATE 共用一个函数，按 TG_OP 扣减旧行、累加新行。
-- 汇总出错只记 WARNING，不影响 usage_logs 写入：计费明细优先于统计。
CREATE OR REPLACE FUNCTION apply_usage_user_daily_rollup()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    BEGIN
        IF TG_OP IN ('DELETE', 'UPDATE') THEN
            UPDATE usage_user_daily_rollups r
            SET requests = r.requests - d.requests,
                input_tokens = r.input_tokens - d.input_tokens,
                output_tokens = r.output_tokens - d.output_tokens,
                cache_creation_tokens = r.cache_creation_tokens - d.cache_creation_tokens,
                cache_read_tokens = r.cache_read_tokens - d.cache_read_tokens,
                image_count = r.image_count - d.image_count,
                video_count = r.video_count - d.video_count,
                actual_cost = r.actual_cost - d.actual_cost
            FROM (
                SELECT o.user_id,
                       (o.created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date,
                       COALESCE(NULLIF(BTRIM(o.requested_model), ''), o.model) AS model,
                       COALESCE(o.group_id, 0) AS group_id,
                       COUNT(*) AS requests,
                       SUM(o.input_tokens) AS input_tokens,
                       SUM(o.output_tokens) AS output_tokens,
                       SUM(o.cache_creation_tokens) AS cache_creation_tokens,
                       SUM(o.cache_read_tokens) AS cache_read_tokens,
                       SUM(COALESCE(o.image_count, 0)) AS image_count,
                       SUM(o.video_count) AS video_count,
                       SUM(o.actual_cost) AS actual_cost
                FROM old_usage_logs o
                WHERE o.billing_type = 0
                GROUP BY 1, 2, 3, 4
            ) d
            WHERE r.user_id = d.user_id
              AND r.bucket_date = d.bucket_date
              AND r.model = d.model
              AND r.group_id = d.group_id;

            DELETE FROM usage_user_daily_rollups r
            USING (
                SELECT DISTINCT o.user_id,
                       (o.created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date,
                       COALESCE(NULLIF(BTRIM(o.requested_model), ''), o.model) AS model,
                       COALESCE(o.group_id, 0) AS group_id
                FROM old_usage_logs o
                WHERE o.billing_type = 0
            ) k
            WHERE r.user_id = k.user_id
              AND r.bucket_date = k.bucket_date
              AND r.model = k.model
              AND r.group_id = k.group_id
              AND r.requests <= 0;
        END IF;

        IF TG_OP IN ('INSERT', 'UPDATE') THEN
            -- GROUP BY 保证同一语句内每个键只出现一次；ORDER BY 让并发批次按相同顺序加锁，避免死锁。
            INSERT INTO usage_user_daily_rollups AS r (
                user_id, bucket_date, model, group_id, requests,
                input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
                image_count, video_count, actual_cost, last_used_at
            )
            SELECT n.user_id,
                   (n.created_at AT TIME ZONE 'Asia/Shanghai')::date,
                   COALESCE(NULLIF(BTRIM(n.requested_model), ''), n.model),
                   COALESCE(n.group_id, 0),
                   COUNT(*),
                   SUM(n.input_tokens),
                   SUM(n.output_tokens),
                   SUM(n.cache_creation_tokens),
                   SUM(n.cache_read_tokens),
                   SUM(COALESCE(n.image_count, 0)),
                   SUM(n.video_count),
                   SUM(n.actual_cost),
                   MAX(n.created_at)
            FROM new_usage_logs n
            WHERE n.billing_type = 0
            GROUP BY 1, 2, 3, 4
            ORDER BY 1, 2, 3, 4
            ON CONFLICT (user_id, bucket_date, model, group_id) DO UPDATE
            SET requests = r.requests + EXCLUDED.requests,
                input_tokens = r.input_tokens + EXCLUDED.input_tokens,
                output_tokens = r.output_tokens + EXCLUDED.output_tokens,
                cache_creation_tokens = r.cache_creation_tokens + EXCLUDED.cache_creation_tokens,
                cache_read_tokens = r.cache_read_tokens + EXCLUDED.cache_read_tokens,
                image_count = r.image_count + EXCLUDED.image_count,
                video_count = r.video_count + EXCLUDED.video_count,
                actual_cost = r.actual_cost + EXCLUDED.actual_cost,
                last_used_at = GREATEST(r.last_used_at, EXCLUDED.last_used_at);
        END IF;
    EXCEPTION WHEN OTHERS THEN
        RAISE WARNING 'usage_user_daily_rollups % failed: % (SQLSTATE %)', TG_OP, SQLERRM, SQLSTATE;
    END;
    RETURN NULL;
END;
$$;

-- 触发器名排在 222 的 usage_logs_group_rollup_* 之后，同事件按名称顺序执行，保持既有锁顺序。
DROP TRIGGER IF EXISTS usage_logs_user_rollup_insert ON usage_logs;
CREATE TRIGGER usage_logs_user_rollup_insert
AFTER INSERT ON usage_logs
REFERENCING NEW TABLE AS new_usage_logs
FOR EACH STATEMENT
EXECUTE FUNCTION apply_usage_user_daily_rollup();

DROP TRIGGER IF EXISTS usage_logs_user_rollup_delete ON usage_logs;
CREATE TRIGGER usage_logs_user_rollup_delete
AFTER DELETE ON usage_logs
REFERENCING OLD TABLE AS old_usage_logs
FOR EACH STATEMENT
EXECUTE FUNCTION apply_usage_user_daily_rollup();

-- 带转换表的触发器不能指定列清单；运行时代码不 UPDATE usage_logs，这里只为保持一致。
DROP TRIGGER IF EXISTS usage_logs_user_rollup_update ON usage_logs;
CREATE TRIGGER usage_logs_user_rollup_update
AFTER UPDATE ON usage_logs
REFERENCING OLD TABLE AS old_usage_logs NEW TABLE AS new_usage_logs
FOR EACH STATEMENT
EXECUTE FUNCTION apply_usage_user_daily_rollup();

-- 按北京时间日期区间重算汇总：同一事务内先删区间再按明细重建。
-- 重建用 ON CONFLICT 累加，与并发写入同时执行时结果仍正确（并发批次的增量会叠加在重建值上，
-- 而重建快照不含这些未提交的行）。运维修复：SELECT rebuild_usage_user_daily_rollups(DATE '...', DATE '...');
CREATE OR REPLACE FUNCTION rebuild_usage_user_daily_rollups(from_date DATE, to_date DATE)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM usage_user_daily_rollups
    WHERE bucket_date BETWEEN from_date AND to_date;

    INSERT INTO usage_user_daily_rollups AS r (
        user_id, bucket_date, model, group_id, requests,
        input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
        image_count, video_count, actual_cost, last_used_at
    )
    SELECT u.user_id,
           (u.created_at AT TIME ZONE 'Asia/Shanghai')::date,
           COALESCE(NULLIF(BTRIM(u.requested_model), ''), u.model),
           COALESCE(u.group_id, 0),
           COUNT(*),
           SUM(u.input_tokens),
           SUM(u.output_tokens),
           SUM(u.cache_creation_tokens),
           SUM(u.cache_read_tokens),
           SUM(COALESCE(u.image_count, 0)),
           SUM(u.video_count),
           SUM(u.actual_cost),
           MAX(u.created_at)
    FROM usage_logs u
    WHERE u.billing_type = 0
      AND u.created_at >= (from_date::timestamp AT TIME ZONE 'Asia/Shanghai')
      AND u.created_at < ((to_date + 1)::timestamp AT TIME ZONE 'Asia/Shanghai')
    GROUP BY 1, 2, 3, 4
    ORDER BY 1, 2, 3, 4
    ON CONFLICT (user_id, bucket_date, model, group_id) DO UPDATE
    SET requests = r.requests + EXCLUDED.requests,
        input_tokens = r.input_tokens + EXCLUDED.input_tokens,
        output_tokens = r.output_tokens + EXCLUDED.output_tokens,
        cache_creation_tokens = r.cache_creation_tokens + EXCLUDED.cache_creation_tokens,
        cache_read_tokens = r.cache_read_tokens + EXCLUDED.cache_read_tokens,
        image_count = r.image_count + EXCLUDED.image_count,
        video_count = r.video_count + EXCLUDED.video_count,
        actual_cost = r.actual_cost + EXCLUDED.actual_cost,
        last_used_at = GREATEST(r.last_used_at, EXCLUDED.last_used_at);
END;
$$;

-- 回填：触发器已建好并持有表锁，此处重算全部历史，重复执行结果一致。
DELETE FROM usage_user_daily_rollups;
SELECT rebuild_usage_user_daily_rollups(DATE '1970-01-01', DATE '9999-12-30');
