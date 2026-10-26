package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const WorkbenchTextModel = "gpt-6.1-sol"
const WorkbenchImageModel = "gpt-image-2.5-sunburst"

// WorkbenchRepository keeps fork-owned signup eligibility and per-user preferences.
// ClaimSignup is an atomic, durable one-shot claim; a failed attempt never recreates a deleted key.
type WorkbenchRepository interface {
	Target(context.Context) (int64, error)
	ClaimSignup(context.Context, int64, int64) (bool, error)
	State(context.Context, int64, string) (*WorkbenchState, error)
	Save(context.Context, int64, string, json.RawMessage, bool) (json.RawMessage, error)
}
type WorkbenchKeyService interface {
	GetAvailableGroups(context.Context, int64) ([]Group, error)
	Create(context.Context, int64, CreateAPIKeyRequest) (*APIKey, error)
}
type WorkbenchState struct {
	Eligible         bool            `json:"eligible"`
	GroupID          int64           `json:"group_id"`
	GroupName        string          `json:"group_name"`
	Initialized      bool            `json:"initialized"`
	Preferences      json.RawMessage `json:"preferences"`
	TextModel        string          `json:"text_model"`
	ImageModel       string          `json:"image_model"`
	ReasoningEffort  string          `json:"reasoning_effort"`
	ReasoningAllowed bool            `json:"reasoning_allowed"`
}

func (s *AuthService) bootstrapWorkbench(ctx context.Context, user *User) {
	if s.workbenchRepo == nil || s.workbenchKeys == nil || user == nil {
		return
	}
	// This auxiliary operation must never turn a successful signup into an error.
	if err := s.createWorkbenchKey(ctx, user.ID); err != nil {
		slog.WarnContext(ctx, "default workbench key initialization failed", "user_id", user.ID, "reason", "INITIALIZATION_FAILED")
	}
}
func (s *AuthService) createWorkbenchKey(ctx context.Context, userID int64) error {
	groupID, err := s.workbenchRepo.Target(ctx)
	if err != nil {
		return err
	}
	claimed, err := s.workbenchRepo.ClaimSignup(ctx, userID, groupID)
	if err != nil || !claimed {
		return err
	}
	if groupID == 0 {
		return errors.New("workbench target group is not configured")
	}
	groups, err := s.workbenchKeys.GetAvailableGroups(ctx, userID)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.ID == groupID && group.IsActive() {
			_, err = s.workbenchKeys.Create(ctx, userID, CreateAPIKeyRequest{Name: group.Name, GroupID: &groupID})
			return err
		}
	}
	return ErrGroupNotAllowed
}
func (s *AuthService) WorkbenchState(ctx context.Context, userID int64, kind string) (*WorkbenchState, error) {
	if kind != "studio" && kind != "chat" {
		return nil, infraerrors.BadRequest("WORKBENCH_INVALID", "invalid workbench")
	}
	if s.workbenchRepo == nil {
		return &WorkbenchState{Preferences: json.RawMessage(`{}`)}, nil
	}
	state, err := s.workbenchRepo.State(ctx, userID, kind)
	if err != nil {
		return nil, err
	}
	state.TextModel, state.ImageModel, state.ReasoningEffort = WorkbenchTextModel, WorkbenchImageModel, "medium"
	if state.GroupID > 0 && s.workbenchKeys != nil {
		groups, err := s.workbenchKeys.GetAvailableGroups(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, group := range groups {
			if group.ID == state.GroupID {
				levels := ChatModelReasoningFor(WorkbenchTextModel, group.MaxReasoningEffort, nil)
				state.ReasoningAllowed = levels != nil && slices.Contains(levels.Levels, "medium")
			}
		}
	}
	return state, nil
}
func (s *AuthService) SaveWorkbench(ctx context.Context, userID int64, kind string, prefs json.RawMessage, initialize bool) (json.RawMessage, error) {
	if kind != "studio" && kind != "chat" {
		return nil, infraerrors.BadRequest("WORKBENCH_INVALID", "invalid workbench")
	}
	// Strict typed whitelist: never persist API key secrets or arbitrary client JSON.
	var value struct {
		Image *struct {
			KeyID int64  `json:"keyId"`
			Model string `json:"model"`
		} `json:"image,omitempty"`
		Text *struct {
			KeyID int64  `json:"keyId"`
			Model string `json:"model"`
		} `json:"text,omitempty"`
		KeyID      *int64 `json:"key_id,omitempty"`
		ImageKeyID *int64 `json:"image_key_id,omitempty"`
	}
	if len(prefs) > 4096 || json.Unmarshal(prefs, &value) != nil {
		return nil, infraerrors.BadRequest("WORKBENCH_PREFERENCES_INVALID", "invalid workbench preferences")
	}
	clean, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if s.workbenchRepo == nil {
		return nil, infraerrors.ServiceUnavailable("WORKBENCH_UNAVAILABLE", "workbench preferences unavailable")
	}
	return s.workbenchRepo.Save(ctx, userID, kind, clean, initialize)
}
