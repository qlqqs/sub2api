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
	groups, err := b.handlers.ModelPlaza.plazaService.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	allowed, restrict, err := b.handlers.APIKey.apiKeyService.GetUserGroupVisibility(ctx, userID)
	if err != nil {
		return nil, err
	}
	groups = filterPlazaVisibleGroups(groups, allowed, restrict)
	rates, err := b.handlers.APIKey.apiKeyService.GetUserGroupRates(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if group.ID == groupID {
			return service.ChatModelPrices(group, rates), nil
		}
	}
	return []service.ChatModelPrice{}, nil
}
