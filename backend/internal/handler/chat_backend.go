package handler

// CUSTOM: narrow adapter for the embedded chat proxy. Keep existing service
// ownership/Wire wiring; never expose service APIKey models to the browser.
import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type ChatBackend struct{ handlers *Handlers }

func NewChatBackend(h *Handlers) *ChatBackend { return &ChatBackend{handlers: h} }

func (b *ChatBackend) Authenticate(ctx context.Context, token string) (*service.User, time.Time, error) {
	claims, err := b.handlers.Auth.authService.ValidateToken(token)
	if err != nil {
		return nil, time.Time{}, err
	}
	user, err := b.handlers.Auth.userService.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, time.Time{}, err
	}
	if !user.IsActive() || user.TokenVersion != claims.TokenVersion || claims.ExpiresAt == nil {
		return nil, time.Time{}, service.ErrInvalidToken
	}
	return user, claims.ExpiresAt.Time, nil
}
func (b *ChatBackend) GetKey(ctx context.Context, id int64) (*service.APIKey, error) {
	return b.handlers.APIKey.apiKeyService.GetByID(ctx, id)
}
func (b *ChatBackend) ListKeys(ctx context.Context, userID int64) ([]service.APIKey, error) {
	out := make([]service.APIKey, 0)
	for page := 1; ; page++ {
		keys, pg, err := b.handlers.APIKey.apiKeyService.List(ctx, userID, pagination.PaginationParams{Page: page, PageSize: 1000, SortBy: "id", SortOrder: pagination.SortOrderAsc}, service.APIKeyListFilters{})
		if err != nil {
			return nil, err
		}
		out = append(out, keys...)
		if pg == nil || page >= pg.Pages || len(keys) == 0 {
			return out, nil
		}
	}
}
func (b *ChatBackend) Groups(ctx context.Context, userID int64) ([]service.Group, error) {
	return b.handlers.APIKey.apiKeyService.GetAvailableGroups(ctx, userID)
}
func (b *ChatBackend) CreateKey(ctx context.Context, userID int64, name string, groupID int64) (*service.APIKey, error) {
	return b.handlers.APIKey.apiKeyService.Create(ctx, userID, service.CreateAPIKeyRequest{Name: name, GroupID: &groupID})
}
func (b *ChatBackend) ModelPrices(ctx context.Context, userID, groupID int64) ([]service.ChatModelPrice, error) {
	group, err := b.plazaGroup(ctx, userID, groupID)
	if err != nil || group == nil {
		return []service.ChatModelPrice{}, err
	}
	rates, err := b.handlers.APIKey.apiKeyService.GetUserGroupRates(ctx, userID)
	if err != nil {
		return nil, err
	}
	return service.ChatModelPrices(*group, rates), nil
}

// ModelReasoning returns effort levels for the listed models under groupID; models
// without a choice are omitted. The group must be available to the user.
func (b *ChatBackend) ModelReasoning(ctx context.Context, userID, groupID int64, models []string) (map[string]*service.ChatModelReasoning, error) {
	out := map[string]*service.ChatModelReasoning{}
	groups, err := b.Groups(ctx, userID)
	if err != nil {
		return nil, err
	}
	maxEffort, found := "", false
	for i := range groups {
		if groups[i].ID == groupID {
			maxEffort, found = groups[i].MaxReasoningEffort, true
			break
		}
	}
	if !found {
		return out, nil
	}
	// Billing multipliers come from the group's channel pricing when there is any.
	pricing := map[string]*service.ChannelModelPricing{}
	if group, err := b.plazaGroup(ctx, userID, groupID); err == nil && group != nil {
		for _, m := range group.Models {
			pricing[m.Name] = m.Pricing
		}
	}
	for _, id := range models {
		if r := service.ChatModelReasoningFor(id, maxEffort, pricing[id]); r != nil {
			out[id] = r
		}
	}
	return out, nil
}

func (b *ChatBackend) plazaGroup(ctx context.Context, userID, groupID int64) (*service.PlazaGroup, error) {
	groups, err := b.handlers.ModelPlaza.plazaService.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	allowed, restrict, err := b.handlers.APIKey.apiKeyService.GetUserGroupVisibility(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, group := range filterPlazaVisibleGroups(groups, allowed, restrict) {
		if group.ID == groupID {
			return &group, nil
		}
	}
	return nil, nil
}

func (b *ChatBackend) WorkbenchState(ctx context.Context, userID int64, kind string) (*service.WorkbenchState, error) {
	return b.handlers.Auth.authService.WorkbenchState(ctx, userID, kind)
}
