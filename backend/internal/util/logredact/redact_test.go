package logredact

import (
	"strings"
	"testing"
)

func TestRedactMap_DefaultBalanceCredentials(t *testing.T) {
	input := map[string]any{
		"credentials": map[string]any{
			"balance_access_token": "balance-token-secret",
			"balance_user_id":      "balance-user-secret",
			"base_url":             "https://upstream.example.com",
		},
	}

	redacted := RedactMap(input)
	credentials, ok := redacted["credentials"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested credentials map, got %#v", redacted["credentials"])
	}
	if credentials["balance_access_token"] != "***" {
		t.Fatalf("expected balance access token redacted, got %#v", credentials["balance_access_token"])
	}
	if credentials["balance_user_id"] != "***" {
		t.Fatalf("expected balance user ID redacted, got %#v", credentials["balance_user_id"])
	}
	if credentials["base_url"] != "https://upstream.example.com" {
		t.Fatalf("expected non-sensitive base URL preserved, got %#v", credentials["base_url"])
	}
}

func TestRedactJSON_DefaultBalanceCredentials(t *testing.T) {
	input := []byte(`{"balance_access_token":"balance-token-secret","nested":{"balance_user_id":"balance-user-secret"},"other":"ok"}`)

	redacted := RedactJSON(input)
	if strings.Contains(redacted, "balance-token-secret") || strings.Contains(redacted, "balance-user-secret") {
		t.Fatalf("expected balance credentials redacted, got %q", redacted)
	}
	if !strings.Contains(redacted, `"balance_access_token":"***"`) {
		t.Fatalf("expected balance access token marker, got %q", redacted)
	}
	if !strings.Contains(redacted, `"balance_user_id":"***"`) {
		t.Fatalf("expected balance user ID marker, got %q", redacted)
	}
}

func TestRedactText_DefaultBalanceCredentials(t *testing.T) {
	input := "balance_access_token=balance-token-secret balance_user_id: balance-user-secret"

	redacted := RedactText(input)
	if strings.Contains(redacted, "balance-token-secret") || strings.Contains(redacted, "balance-user-secret") {
		t.Fatalf("expected balance credentials redacted, got %q", redacted)
	}
	if !strings.Contains(redacted, "balance_access_token=***") {
		t.Fatalf("expected balance access token marker, got %q", redacted)
	}
	if !strings.Contains(redacted, "balance_user_id: ***") {
		t.Fatalf("expected balance user ID marker, got %q", redacted)
	}
}

func TestRedactText_JSONLike(t *testing.T) {
	in := `{"access_token":"ya29.a0AfH6SMDUMMY","refresh_token":"1//0gDUMMY","other":"ok"}`
	out := RedactText(in)
	if out == in {
		t.Fatalf("expected redaction, got unchanged")
	}
	if want := `"access_token":"***"`; !strings.Contains(out, want) {
		t.Fatalf("expected %q in %q", want, out)
	}
	if want := `"refresh_token":"***"`; !strings.Contains(out, want) {
		t.Fatalf("expected %q in %q", want, out)
	}
}

func TestRedactText_QueryLike(t *testing.T) {
	in := "access_token=ya29.a0AfH6SMDUMMY refresh_token=1//0gDUMMY"
	out := RedactText(in)
	if strings.Contains(out, "ya29") || strings.Contains(out, "1//0") {
		t.Fatalf("expected tokens redacted, got %q", out)
	}
}

func TestRedactText_GOCSPX(t *testing.T) {
	in := "client_secret=GOCSPX-your-client-secret"
	out := RedactText(in)
	if strings.Contains(out, "your-client-secret") {
		t.Fatalf("expected secret redacted, got %q", out)
	}
	if !strings.Contains(out, "client_secret=***") {
		t.Fatalf("expected key redacted, got %q", out)
	}
}

func TestRedactText_ExtraKeyCacheUsesNormalizedSortedKey(t *testing.T) {
	clearExtraTextPatternCache()

	out1 := RedactText("custom_secret=abc", "Custom_Secret", " custom_secret ")
	out2 := RedactText("custom_secret=xyz", "custom_secret")
	if !strings.Contains(out1, "custom_secret=***") {
		t.Fatalf("expected custom key redacted in first call, got %q", out1)
	}
	if !strings.Contains(out2, "custom_secret=***") {
		t.Fatalf("expected custom key redacted in second call, got %q", out2)
	}

	if got := countExtraTextPatternCacheEntries(); got != 1 {
		t.Fatalf("expected 1 cached pattern set, got %d", got)
	}
}

func TestRedactText_DefaultPathDoesNotUseExtraCache(t *testing.T) {
	clearExtraTextPatternCache()

	out := RedactText("access_token=abc")
	if !strings.Contains(out, "access_token=***") {
		t.Fatalf("expected default key redacted, got %q", out)
	}
	if got := countExtraTextPatternCacheEntries(); got != 0 {
		t.Fatalf("expected extra cache to remain empty, got %d", got)
	}
}

func clearExtraTextPatternCache() {
	extraTextPatternCache.Range(func(key, value any) bool {
		extraTextPatternCache.Delete(key)
		return true
	})
}

func countExtraTextPatternCacheEntries() int {
	count := 0
	extraTextPatternCache.Range(func(key, value any) bool {
		count++
		return true
	})
	return count
}
