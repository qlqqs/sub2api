//go:build unit

package admin

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSettingsGroupRebateFactorsPresenceAndAudit(t *testing.T) {
	var omitted, empty, supplied UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.Nil(t, omitted.AffiliateGroupRebateFactors)
	require.NoError(t, json.Unmarshal([]byte(`{"affiliate_group_rebate_factors":[]}`), &empty))
	require.NotNil(t, empty.AffiliateGroupRebateFactors)
	require.Empty(t, *empty.AffiliateGroupRebateFactors)
	require.NoError(t, json.Unmarshal([]byte(`{"affiliate_group_rebate_factors":[{"group_id":12,"factor":30,"group_name":"ignored"}]}`), &supplied))
	require.Equal(t, []service.AffiliateGroupRebateFactor{{GroupID: 12, Factor: 30}}, *supplied.AffiliateGroupRebateFactors)
	before := &service.SystemSettings{AffiliateGroupRebateFactors: []service.AffiliateGroupRebateFactor{{GroupID: 12, Factor: 30}}}
	after := &service.SystemSettings{AffiliateGroupRebateFactors: []service.AffiliateGroupRebateFactor{}}
	require.Contains(t, diffSettings(before, after, nil, nil, empty), "affiliate_group_rebate_factors")
	require.NotContains(t, diffSettings(before, after, nil, nil, omitted), "affiliate_group_rebate_factors")
}
