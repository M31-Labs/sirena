package svg

import (
	"errors"
	"testing"
)

func TestTheme_Default(t *testing.T) {
	th, err := ThemeForName("earth-default")
	if err != nil {
		t.Fatalf("ThemeForName(earth-default) error: %v", err)
	}
	if th == nil {
		t.Fatal("earth-default theme is nil")
	}
	for token := range AllowlistedTokens {
		if _, ok := th.Tokens[token]; !ok {
			t.Errorf("earth-default missing a value for %s", token)
		}
	}
}

func TestTheme_UnknownName(t *testing.T) {
	th, err := ThemeForName("not-a-theme")
	if th != nil {
		t.Errorf("unknown theme returned non-nil: %+v", th)
	}
	if !errors.Is(err, ErrUnknownTheme) {
		t.Errorf("err = %v, want ErrUnknownTheme", err)
	}
}

// TestTheme_RegistryComplete enforces the theme contract across EVERY registered
// theme: each must define a value for every allowlisted token (completeness) and
// must emit nothing outside the allowlist (safety invariant 9). Iterating the
// registry means a newly added theme (e.g. midnight) is held to the same bar
// without touching this test.
func TestTheme_RegistryComplete(t *testing.T) {
	for name, theme := range themes {
		for token := range AllowlistedTokens {
			if _, ok := theme.Tokens[token]; !ok {
				t.Errorf("theme %q missing a value for %s", name, token)
			}
		}
		for token := range theme.Tokens {
			if _, ok := AllowlistedTokens[token]; !ok {
				t.Errorf("theme %q emits non-allowlisted token %s", name, token)
			}
		}
	}
}

func TestTheme_Midnight(t *testing.T) {
	th, err := ThemeForName("midnight")
	if err != nil {
		t.Fatalf("ThemeForName(midnight) error: %v", err)
	}
	if th == nil || th.Name != "midnight" {
		t.Fatalf("midnight theme not registered correctly: %+v", th)
	}
}
