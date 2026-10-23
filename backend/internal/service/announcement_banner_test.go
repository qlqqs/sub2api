//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type bannerSettingRepo struct {
	SettingRepository
	values map[string]string
	getErr error
}

func (r *bannerSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if r.getErr != nil {
		return "", r.getErr
	}
	v, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}

func (r *bannerSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func TestAnnouncementBannerService_DefaultsToEmptyLists(t *testing.T) {
	svc := NewAnnouncementBannerService(&bannerSettingRepo{values: map[string]string{}})
	cfg, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, AnnouncementBannerConfig{Order: []int64{}, Hidden: []int64{}}, cfg)
}

func TestAnnouncementBannerService_UpdateNormalizesAndPersists(t *testing.T) {
	repo := &bannerSettingRepo{values: map[string]string{}}
	svc := NewAnnouncementBannerService(repo)
	cfg, err := svc.Update(context.Background(), AnnouncementBannerConfig{
		Order:  []int64{3, 0, 1, 3, -2},
		Hidden: []int64{1, 5, 5},
	})
	require.NoError(t, err)
	// 去掉非正数与重复；同时出现在两边的以 Order 为准
	require.Equal(t, AnnouncementBannerConfig{Order: []int64{3, 1}, Hidden: []int64{5}}, cfg)
	require.JSONEq(t, `{"order":[3,1],"hidden":[5]}`, repo.values[SettingKeyDashboardAnnouncementBanner])

	got, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, cfg, got)
}

func TestAnnouncementBannerService_RejectsOversizedLists(t *testing.T) {
	svc := NewAnnouncementBannerService(&bannerSettingRepo{values: map[string]string{}})
	_, err := svc.Update(context.Background(), AnnouncementBannerConfig{Order: make([]int64, announcementBannerMaxIDs+1)})
	require.Error(t, err)
}

func TestAnnouncementBannerService_CorruptValueFallsBackToDefault(t *testing.T) {
	svc := NewAnnouncementBannerService(&bannerSettingRepo{values: map[string]string{SettingKeyDashboardAnnouncementBanner: "{oops"}})
	cfg, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.Empty(t, cfg.Order)
	require.Empty(t, cfg.Hidden)
}

func TestAnnouncementBannerService_PropagatesRepoErrors(t *testing.T) {
	svc := NewAnnouncementBannerService(&bannerSettingRepo{values: map[string]string{}, getErr: errors.New("db down")})
	_, err := svc.Get(context.Background())
	require.Error(t, err)
}
