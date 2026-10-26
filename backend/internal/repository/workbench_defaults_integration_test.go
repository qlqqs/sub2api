//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkbenchDefaultsPersistentIsolationAndCAS(t *testing.T) {
	ctx := context.Background()
	_ = testEntClient(t)
	var uid int64
	err := integrationDB.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role,status) VALUES ($1,'test','user','active') RETURNING id`, fmt.Sprintf("workbench-%d@test.invalid", time.Now().UnixNano())).Scan(&uid)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, uid) })
	repo := NewWorkbenchRepository(integrationDB)
	before, err := repo.State(ctx, uid, "studio")
	require.NoError(t, err)
	require.False(t, before.Eligible)
	var won atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, e := repo.ClaimSignup(ctx, uid, 0)
			if e != nil {
				t.Error(e)
			}
			if ok {
				won.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, won.Load())
	_, err = repo.Save(ctx, uid, "studio", json.RawMessage(`{}`), false)
	require.NoError(t, err)
	prefs, err := repo.Save(ctx, uid, "studio", json.RawMessage(`{"text":{"keyId":1,"model":"gpt-6.1-sol"}}`), true)
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(prefs), "explicit clear survives initialization")
	studio, err := repo.State(ctx, uid, "studio")
	require.NoError(t, err)
	require.True(t, studio.Initialized)
	require.True(t, studio.Eligible)
	chat, err := repo.State(ctx, uid, "chat")
	require.NoError(t, err)
	require.False(t, chat.Initialized)
}
