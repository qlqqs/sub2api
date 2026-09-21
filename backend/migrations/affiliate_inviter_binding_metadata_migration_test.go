package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAffiliateInviterBindingMetadataMigration(t *testing.T) {
	content, err := FS.ReadFile("239_affiliate_inviter_binding_metadata.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{
		"inviter_bound_at timestamptz",
		"inviter_bound_source varchar(32)",
		"add column if not exists inviter_bound_by bigint",
		"user_affiliates_inviter_bound_by_fkey",
		"foreign key (inviter_bound_by) references users(id) on delete set null",
		"inviter_bind_reason varchar(500)",
		"set inviter_bound_at = coalesce(inviter_bound_at, created_at)",
		"inviter_bound_source = coalesce(inviter_bound_source, 'registration')",
		"where inviter_id is not null",
		"create or replace function enforce_user_affiliate_inviter_binding()",
		"new.inviter_bound_at := coalesce(new.inviter_bound_at, now())",
		"new.inviter_bound_source := coalesce(new.inviter_bound_source, 'registration')",
		"new.inviter_bound_at := old.inviter_bound_at",
		"new.inviter_id is distinct from old.inviter_id",
		"raise exception 'affiliate inviter has already been bound",
		"drop trigger if exists user_affiliates_inviter_binding_guard",
		"before insert or update of inviter_id, inviter_bound_at, inviter_bound_source",
		"execute function enforce_user_affiliate_inviter_binding()",
	} {
		require.Contains(t, sql, fragment)
	}
}
