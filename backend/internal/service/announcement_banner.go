package service

// CUSTOM(fork): 仪表盘公告轮播的名单与顺序，存在设置表的一个键里，不改公告表结构。
//
// 规则：Order 里的公告按顺序排在最前；Hidden 里的公告不上轮播；
// 两边都没有的（例如新发布的公告）默认上轮播，排在 Order 之后，保持公告列表原有顺序。
// 用户端拿到的只是 ID 名单，具体哪些公告可见仍由公告本身的状态、时间和展示条件决定。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// SettingKeyDashboardAnnouncementBanner 仪表盘公告轮播配置（JSON：{"order":[...],"hidden":[...]}）。
const SettingKeyDashboardAnnouncementBanner = "dashboard_announcement_banner"

// announcementBannerMaxIDs 名单上限，防止异常请求写入超大配置。
const announcementBannerMaxIDs = 200

// AnnouncementBannerConfig 公告轮播配置。
type AnnouncementBannerConfig struct {
	Order  []int64 `json:"order"`
	Hidden []int64 `json:"hidden"`
}

// AnnouncementBannerService 读写公告轮播配置。
type AnnouncementBannerService struct {
	settingRepo SettingRepository
}

// NewAnnouncementBannerService 创建公告轮播配置服务。
func NewAnnouncementBannerService(settingRepo SettingRepository) *AnnouncementBannerService {
	return &AnnouncementBannerService{settingRepo: settingRepo}
}

// Get 返回当前配置；从未设置过时返回空名单（全部公告按默认顺序上轮播）。
func (s *AnnouncementBannerService) Get(ctx context.Context) (AnnouncementBannerConfig, error) {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyDashboardAnnouncementBanner)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return AnnouncementBannerConfig{Order: []int64{}, Hidden: []int64{}}, nil
		}
		return AnnouncementBannerConfig{}, fmt.Errorf("get announcement banner setting: %w", err)
	}
	var cfg AnnouncementBannerConfig
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			// 配置损坏时按未设置处理，不影响仪表盘
			return AnnouncementBannerConfig{Order: []int64{}, Hidden: []int64{}}, nil
		}
	}
	return normalizeAnnouncementBannerConfig(cfg), nil
}

// Update 保存配置：去掉非正数和重复 ID；同一 ID 同时出现在两边时以 Order 为准。
func (s *AnnouncementBannerService) Update(ctx context.Context, cfg AnnouncementBannerConfig) (AnnouncementBannerConfig, error) {
	if len(cfg.Order) > announcementBannerMaxIDs || len(cfg.Hidden) > announcementBannerMaxIDs {
		return AnnouncementBannerConfig{}, infraerrors.BadRequest("ANNOUNCEMENT_BANNER_TOO_MANY", fmt.Sprintf("轮播名单最多 %d 条", announcementBannerMaxIDs))
	}
	cfg = normalizeAnnouncementBannerConfig(cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return AnnouncementBannerConfig{}, fmt.Errorf("marshal announcement banner setting: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyDashboardAnnouncementBanner, string(raw)); err != nil {
		return AnnouncementBannerConfig{}, fmt.Errorf("save announcement banner setting: %w", err)
	}
	return cfg, nil
}

func normalizeAnnouncementBannerConfig(cfg AnnouncementBannerConfig) AnnouncementBannerConfig {
	seen := make(map[int64]struct{}, len(cfg.Order)+len(cfg.Hidden))
	pick := func(ids []int64) []int64 {
		out := make([]int64, 0, len(ids))
		for _, id := range ids {
			if id <= 0 {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
		return out
	}
	order := pick(cfg.Order)
	hidden := pick(cfg.Hidden)
	return AnnouncementBannerConfig{Order: order, Hidden: hidden}
}
