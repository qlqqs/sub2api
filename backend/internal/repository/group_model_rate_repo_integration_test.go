//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// CUSTOM: 分组模型倍率仓储（迁移 253 的 group_model_rate_multipliers）。
func TestGroupModelRateRepository_ReplaceListCopyAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewGroupModelRateRepository(integrationDB)
	suffix := time.Now().UnixNano()
	g1 := mustCreateGroup(t, client, &service.Group{Name: fmt.Sprintf("gmr-a-%d", suffix), RateMultiplier: 1})
	g2 := mustCreateGroup(t, client, &service.Group{Name: fmt.Sprintf("gmr-b-%d", suffix), RateMultiplier: 1})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM group_model_rate_multipliers WHERE group_id = ANY($1)`, fmt.Sprintf("{%d,%d}", g1.ID, g2.ID))
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id = ANY($1)`, fmt.Sprintf("{%d,%d}", g1.ID, g2.ID))
	})

	exists, err := repo.GroupExists(ctx, g1.ID)
	require.NoError(t, err)
	require.True(t, exists)

	// 整组替换 + DECIMAL(10,4) 落库精度 + 按 model 排序。
	require.NoError(t, repo.ReplaceForGroup(ctx, g1.ID, []service.GroupModelRateRule{
		{Model: "claude-opus-*", RateMultiplier: 2},
		{Model: "a-model", RateMultiplier: 1.23456},
	}))
	rules, err := repo.ListByGroup(ctx, g1.ID)
	require.NoError(t, err)
	require.Equal(t, []service.GroupModelRateRule{
		{Model: "a-model", RateMultiplier: 1.2346},
		{Model: "claude-opus-*", RateMultiplier: 2},
	}, rules)

	require.NoError(t, repo.ReplaceForGroup(ctx, g1.ID, []service.GroupModelRateRule{{Model: "b-model", RateMultiplier: 3}}))
	rules, err = repo.ListByGroup(ctx, g1.ID)
	require.NoError(t, err)
	require.Equal(t, []service.GroupModelRateRule{{Model: "b-model", RateMultiplier: 3}}, rules, "整组替换，旧规则删除")

	// 事务：插入失败时先删的旧规则回滚。
	err = repo.ReplaceForGroup(ctx, g1.ID, []service.GroupModelRateRule{
		{Model: "dup", RateMultiplier: 1},
		{Model: "dup", RateMultiplier: 2},
	})
	require.Error(t, err)
	require.Equal(t, "GROUP_MODEL_RATE_DUPLICATE", infraerrors.Reason(err))
	err = repo.ReplaceForGroup(ctx, g1.ID, []service.GroupModelRateRule{{Model: "zero", RateMultiplier: 0}})
	require.Error(t, err, "CHECK (rate_multiplier > 0)")
	rules, err = repo.ListByGroup(ctx, g1.ID)
	require.NoError(t, err)
	require.Equal(t, []service.GroupModelRateRule{{Model: "b-model", RateMultiplier: 3}}, rules, "失败的替换整体回滚")

	// 不存在的分组。
	err = repo.ReplaceForGroup(ctx, 9_000_000_000+suffix%1000, []service.GroupModelRateRule{{Model: "a", RateMultiplier: 1}})
	require.ErrorIs(t, err, service.ErrGroupNotFound)

	// 复制到新分组（已有同名规则时覆盖倍率）。
	require.NoError(t, repo.ReplaceForGroup(ctx, g2.ID, []service.GroupModelRateRule{{Model: "b-model", RateMultiplier: 9}, {Model: "keep", RateMultiplier: 1.5}}))
	require.NoError(t, repo.CopyGroup(ctx, g1.ID, g2.ID))
	rules, err = repo.ListByGroup(ctx, g2.ID)
	require.NoError(t, err)
	require.Equal(t, []service.GroupModelRateRule{{Model: "b-model", RateMultiplier: 3}, {Model: "keep", RateMultiplier: 1.5}}, rules)

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Equal(t, []service.GroupModelRateRule{{Model: "b-model", RateMultiplier: 3}}, all[g1.ID])
	require.Len(t, all[g2.ID], 2)

	// 软删除分组：快照与管理端列表都不再包含其规则，替换返回 404。
	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET deleted_at = NOW() WHERE id = $1`, g2.ID)
	require.NoError(t, err)
	all, err = repo.ListAll(ctx)
	require.NoError(t, err)
	_, hasDeleted := all[g2.ID]
	require.False(t, hasDeleted)
	exists, err = repo.GroupExists(ctx, g2.ID)
	require.NoError(t, err)
	require.False(t, exists)
	require.ErrorIs(t, repo.ReplaceForGroup(ctx, g2.ID, nil), service.ErrGroupNotFound)

	// 硬删除分组级联删除规则。
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id = $1`, g1.ID)
	require.NoError(t, err)
	var remaining int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_model_rate_multipliers WHERE group_id = $1`, g1.ID).Scan(&remaining))
	require.Zero(t, remaining)
}

// 服务层经真实仓储：保存后本进程立即生效，匹配走归一化 + 最长前缀。
func TestGroupModelRateService_WithRepository(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	group := mustCreateGroup(t, client, &service.Group{Name: fmt.Sprintf("gmr-svc-%d", time.Now().UnixNano()), RateMultiplier: 1})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM group_model_rate_multipliers WHERE group_id = $1`, group.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id = $1`, group.ID)
	})
	svc := service.NewGroupModelRateService(NewGroupModelRateRepository(integrationDB))

	_, ok := svc.Match(ctx, group.ID, "claude-opus-4-5")
	require.False(t, ok)
	saved, err := svc.ReplaceForGroup(ctx, group.ID, []service.GroupModelRateRule{
		{Model: "Claude-*", RateMultiplier: 1.5},
		{Model: "claude-opus-*", RateMultiplier: 2},
	})
	require.NoError(t, err)
	require.Len(t, saved, 2)
	rate, ok := svc.Match(ctx, group.ID, "Claude-Opus-4.5")
	require.True(t, ok)
	require.InDelta(t, 2.0, rate, 1e-12)
	rate, ok = svc.Match(ctx, group.ID, "claude-sonnet-4")
	require.True(t, ok)
	require.InDelta(t, 1.5, rate, 1e-12)
}
