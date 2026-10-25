package admin

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// normalizeClientDownloads validates the admin input and trims it before storage.
// Indices in errors are one-based so administrators can locate the invalid entry.
func normalizeClientDownloads(items []dto.ClientDownload) error {
	invalid := func(field, message string) error {
		return infraerrors.BadRequest("CLIENT_DOWNLOADS_INVALID", field+": "+message)
	}
	if len(items) > 20 {
		return invalid("client_downloads", "at most 20 clients are allowed")
	}
	validateLength := func(field, value string, min, max int) error {
		if n := utf8.RuneCountInString(value); n < min || n > max {
			return invalid(field, fmt.Sprintf("must contain %d..%d characters", min, max))
		}
		return nil
	}
	validateURL := func(field, value string, optional bool) error {
		if optional && value == "" {
			return nil
		}
		if err := validateLength(field, value, 1, 2048); err != nil {
			return err
		}
		if err := config.ValidateAbsoluteHTTPURL(value); err != nil {
			return invalid(field, "must be an absolute http(s) URL")
		}
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	for i := range items {
		item := &items[i]
		prefix := fmt.Sprintf("client_downloads client %d", i+1)
		item.ID = strings.TrimSpace(item.ID)
		item.Name = strings.TrimSpace(item.Name)
		item.Description = strings.TrimSpace(item.Description)
		item.IconURL = strings.TrimSpace(item.IconURL)
		item.Version = strings.TrimSpace(item.Version)
		item.HomepageURL = strings.TrimSpace(item.HomepageURL)
		if item.ID == "" {
			id, err := generateMenuItemID()
			if err != nil {
				return fmt.Errorf("generate client download ID: %w", err)
			}
			item.ID = id
		}
		if len(item.ID) > 32 || !menuItemIDPattern.MatchString(item.ID) {
			return invalid(prefix+" id", "must match [a-zA-Z0-9_-]{1,32}")
		}
		if _, exists := seen[item.ID]; exists {
			return invalid(prefix+" id", "must be unique")
		}
		seen[item.ID] = struct{}{}
		if err := validateLength(prefix+" name", item.Name, 1, 50); err != nil {
			return err
		}
		if err := validateLength(prefix+" description", item.Description, 0, 300); err != nil {
			return err
		}
		if err := validateLength(prefix+" version", item.Version, 0, 32); err != nil {
			return err
		}
		if err := validateURL(prefix+" icon_url", item.IconURL, true); err != nil {
			return err
		}
		if err := validateURL(prefix+" homepage_url", item.HomepageURL, true); err != nil {
			return err
		}
		if len(item.Links) < 1 || len(item.Links) > 20 {
			return invalid(prefix+" links", "must contain 1..20 links")
		}
		for j := range item.Links {
			link := &item.Links[j]
			linkPrefix := fmt.Sprintf("%s link %d", prefix, j+1)
			link.Label = strings.TrimSpace(link.Label)
			link.Platform = strings.TrimSpace(link.Platform)
			link.Arch = strings.TrimSpace(link.Arch)
			link.URL = strings.TrimSpace(link.URL)
			if err := validateLength(linkPrefix+" label", link.Label, 1, 50); err != nil {
				return err
			}
			switch link.Platform {
			case "windows", "macos", "linux", "android", "ios", "other":
			default:
				return invalid(linkPrefix+" platform", "must be windows, macos, linux, android, ios or other")
			}
			switch link.Arch {
			case "", "x64", "arm64", "x86", "universal":
			default:
				return invalid(linkPrefix+" arch", "must be empty, x64, arm64, x86 or universal")
			}
			if err := validateURL(linkPrefix+" url", link.URL, false); err != nil {
				return err
			}
		}
	}
	return nil
}
