-- 管理端团队消费页：按受益代理 × 来源客户 × 北京时间自然日汇总代理分润（agent_direct / agent_diff）。
-- 汇总页要一次聚合全部代理在 90 天内的分润；流水条数随结算频率增长且永久保存，汇总表只与「有分润的客户 × 天数」有关。
-- 由 user_affiliate_ledger 上的语句级触发器增量维护；迁移内先建触发器再回填，同一事务内完成。

CREATE TABLE IF NOT EXISTS affiliate_agent_reward_daily_rollups (
    user_id BIGINT NOT NULL,
    source_user_id BIGINT NOT NULL DEFAULT 0,
    bucket_date DATE NOT NULL,
    amount DECIMAL(20, 8) NOT NULL DEFAULT 0,
    entries BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, source_user_id, bucket_date)
);

CREATE INDEX IF NOT EXISTS idx_affiliate_agent_reward_daily_rollups_bucket_date
    ON affiliate_agent_reward_daily_rollups (bucket_date);

COMMENT ON TABLE affiliate_agent_reward_daily_rollups IS '按受益代理、来源客户、北京时间自然日汇总的代理分润（agent_direct / agent_diff），由 user_affiliate_ledger 触发器维护。';
COMMENT ON COLUMN affiliate_agent_reward_daily_rollups.user_id IS '受益代理（流水的 user_id）。';
COMMENT ON COLUMN affiliate_agent_reward_daily_rollups.source_user_id IS '来源客户，0 表示来源账号已被物理删除（流水里为 NULL）。';
COMMENT ON COLUMN affiliate_agent_reward_daily_rollups.bucket_date IS '入账时间所在的北京时间自然日。';
COMMENT ON COLUMN affiliate_agent_reward_daily_rollups.entries IS '流水条数，扣减到 0 时删除该行。';

-- INSERT / DELETE / UPDATE 共用一个函数，全部写入都走按主键排序的 upsert（扣减即累加负数），
-- 并发批次按相同顺序加锁，避免死锁。汇总出错只记 WARNING，不影响流水写入：入账优先于统计。
CREATE OR REPLACE FUNCTION apply_affiliate_agent_reward_daily_rollup()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    BEGIN
        IF TG_OP = 'INSERT' THEN
            -- GROUP BY 保证同一语句内每个键只出现一次。
            INSERT INTO affiliate_agent_reward_daily_rollups AS r (user_id, source_user_id, bucket_date, amount, entries)
            SELECT n.user_id,
                   COALESCE(n.source_user_id, 0),
                   (n.created_at AT TIME ZONE 'Asia/Shanghai')::date,
                   SUM(n.amount),
                   COUNT(*)
            FROM new_ledger n
            WHERE n.reward_type IN ('agent_direct', 'agent_diff')
            GROUP BY 1, 2, 3
            ORDER BY 1, 2, 3
            ON CONFLICT (user_id, source_user_id, bucket_date) DO UPDATE
            SET amount = r.amount + EXCLUDED.amount,
                entries = r.entries + EXCLUDED.entries;

        ELSIF TG_OP = 'DELETE' THEN
            -- 受益人被物理删除时外键级联删除流水，也经过这里。
            INSERT INTO affiliate_agent_reward_daily_rollups AS r (user_id, source_user_id, bucket_date, amount, entries)
            SELECT o.user_id,
                   COALESCE(o.source_user_id, 0),
                   (o.created_at AT TIME ZONE 'Asia/Shanghai')::date,
                   -SUM(o.amount),
                   -COUNT(*)
            FROM old_ledger o
            WHERE o.reward_type IN ('agent_direct', 'agent_diff')
            GROUP BY 1, 2, 3
            ORDER BY 1, 2, 3
            ON CONFLICT (user_id, source_user_id, bucket_date) DO UPDATE
            SET amount = r.amount + EXCLUDED.amount,
                entries = r.entries + EXCLUDED.entries;

            DELETE FROM affiliate_agent_reward_daily_rollups r
            USING (
                SELECT DISTINCT o.user_id,
                       COALESCE(o.source_user_id, 0) AS source_user_id,
                       (o.created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date
                FROM old_ledger o
                WHERE o.reward_type IN ('agent_direct', 'agent_diff')
            ) k
            WHERE r.user_id = k.user_id
              AND r.source_user_id = k.source_user_id
              AND r.bucket_date = k.bucket_date
              AND r.entries <= 0;

        ELSE
            -- UPDATE：按键合并「旧行扣减、新行累加」，只写净变化不为 0 的键。
            -- 解冻（frozen_until）、提现回写快照列等不改汇总列的 UPDATE 净变化为 0，不写汇总；
            -- 来源用户被物理删除时外键 SET NULL，金额从原来源挪到来源 0。
            INSERT INTO affiliate_agent_reward_daily_rollups AS r (user_id, source_user_id, bucket_date, amount, entries)
            SELECT d.user_id, d.source_user_id, d.bucket_date, SUM(d.amount), SUM(d.entries)
            FROM (
                SELECT o.user_id,
                       COALESCE(o.source_user_id, 0) AS source_user_id,
                       (o.created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date,
                       -o.amount AS amount,
                       -1 AS entries
                FROM old_ledger o
                WHERE o.reward_type IN ('agent_direct', 'agent_diff')
                UNION ALL
                SELECT n.user_id,
                       COALESCE(n.source_user_id, 0),
                       (n.created_at AT TIME ZONE 'Asia/Shanghai')::date,
                       n.amount,
                       1
                FROM new_ledger n
                WHERE n.reward_type IN ('agent_direct', 'agent_diff')
            ) d
            GROUP BY 1, 2, 3
            HAVING SUM(d.amount) <> 0 OR SUM(d.entries) <> 0
            ORDER BY 1, 2, 3
            ON CONFLICT (user_id, source_user_id, bucket_date) DO UPDATE
            SET amount = r.amount + EXCLUDED.amount,
                entries = r.entries + EXCLUDED.entries;

            DELETE FROM affiliate_agent_reward_daily_rollups r
            USING (
                SELECT d.user_id, d.source_user_id, d.bucket_date
                FROM (
                    SELECT o.user_id,
                           COALESCE(o.source_user_id, 0) AS source_user_id,
                           (o.created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date,
                           -1 AS entries
                    FROM old_ledger o
                    WHERE o.reward_type IN ('agent_direct', 'agent_diff')
                    UNION ALL
                    SELECT n.user_id,
                           COALESCE(n.source_user_id, 0),
                           (n.created_at AT TIME ZONE 'Asia/Shanghai')::date,
                           1
                    FROM new_ledger n
                    WHERE n.reward_type IN ('agent_direct', 'agent_diff')
                ) d
                GROUP BY 1, 2, 3
                HAVING SUM(d.entries) < 0
            ) k
            WHERE r.user_id = k.user_id
              AND r.source_user_id = k.source_user_id
              AND r.bucket_date = k.bucket_date
              AND r.entries <= 0;
        END IF;
    EXCEPTION WHEN OTHERS THEN
        RAISE WARNING 'affiliate_agent_reward_daily_rollups % failed: % (SQLSTATE %)', TG_OP, SQLERRM, SQLSTATE;
    END;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS user_affiliate_ledger_reward_rollup_insert ON user_affiliate_ledger;
CREATE TRIGGER user_affiliate_ledger_reward_rollup_insert
AFTER INSERT ON user_affiliate_ledger
REFERENCING NEW TABLE AS new_ledger
FOR EACH STATEMENT
EXECUTE FUNCTION apply_affiliate_agent_reward_daily_rollup();

DROP TRIGGER IF EXISTS user_affiliate_ledger_reward_rollup_delete ON user_affiliate_ledger;
CREATE TRIGGER user_affiliate_ledger_reward_rollup_delete
AFTER DELETE ON user_affiliate_ledger
REFERENCING OLD TABLE AS old_ledger
FOR EACH STATEMENT
EXECUTE FUNCTION apply_affiliate_agent_reward_daily_rollup();

-- 带转换表的触发器不能指定列清单，由函数内按净变化过滤。
DROP TRIGGER IF EXISTS user_affiliate_ledger_reward_rollup_update ON user_affiliate_ledger;
CREATE TRIGGER user_affiliate_ledger_reward_rollup_update
AFTER UPDATE ON user_affiliate_ledger
REFERENCING OLD TABLE AS old_ledger NEW TABLE AS new_ledger
FOR EACH STATEMENT
EXECUTE FUNCTION apply_affiliate_agent_reward_daily_rollup();

-- 按北京时间日期区间重算汇总：同一事务内先删区间再按流水重建。
-- 先以 SHARE 锁住流水表，并发的流水写入等到本事务结束：删除与重建看到同一份已提交数据，
-- 两步之间提交的新键不会被重复累加。锁持续到事务结束，区间越大等待越久。
-- 运维修复：SELECT rebuild_affiliate_agent_reward_daily_rollups(DATE '...', DATE '...');
CREATE OR REPLACE FUNCTION rebuild_affiliate_agent_reward_daily_rollups(from_date DATE, to_date DATE)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    LOCK TABLE user_affiliate_ledger IN SHARE MODE;

    DELETE FROM affiliate_agent_reward_daily_rollups
    WHERE bucket_date BETWEEN from_date AND to_date;

    INSERT INTO affiliate_agent_reward_daily_rollups AS r (user_id, source_user_id, bucket_date, amount, entries)
    SELECT l.user_id,
           COALESCE(l.source_user_id, 0),
           (l.created_at AT TIME ZONE 'Asia/Shanghai')::date,
           SUM(l.amount),
           COUNT(*)
    FROM user_affiliate_ledger l
    WHERE l.reward_type IN ('agent_direct', 'agent_diff')
      AND l.created_at >= (from_date::timestamp AT TIME ZONE 'Asia/Shanghai')
      AND l.created_at < ((to_date + 1)::timestamp AT TIME ZONE 'Asia/Shanghai')
    GROUP BY 1, 2, 3
    ORDER BY 1, 2, 3
    ON CONFLICT (user_id, source_user_id, bucket_date) DO UPDATE
    SET amount = r.amount + EXCLUDED.amount,
        entries = r.entries + EXCLUDED.entries;
END;
$$;

-- 回填：触发器已建好并持有表锁，此处重算全部历史，重复执行结果一致。
DELETE FROM affiliate_agent_reward_daily_rollups;
SELECT rebuild_affiliate_agent_reward_daily_rollups(DATE '1970-01-01', DATE '9999-12-30');
