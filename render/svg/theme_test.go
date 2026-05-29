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

func TestTheme_AllTokensAllowlisted(t *testing.T) {
	for _, th := range []string{"earth-default"} {
		theme, _ := ThemeForName(th)
		for token := range theme.Tokens {
			if _, ok := AllowlistedTokens[token]; !ok {
				t.Errorf("theme %q emits non-allowlisted token %s", th, token)
			}
		}
	}
}
