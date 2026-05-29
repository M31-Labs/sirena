// Package svg renders a sirena layout to deterministic, self-contained
// SVG: bundled-font glyph paths instead of <text>, an allowlisted set of
// CSS-variable design tokens, and a fixed element ordering so the same
// layout always produces byte-identical output.
package svg

import "errors"

// ErrUnknownTheme is returned by ThemeForName for a name with no
// registered theme.
var ErrUnknownTheme = errors.New("svg: unknown theme")

// AllowlistedTokens is the fixed set of CSS-variable names a theme may
// emit (spec safety invariant 9). A theme whose Tokens stray outside this
// set is a bug; the allowlist only grows through this file, under review.
var AllowlistedTokens = map[string]struct{}{
	"--sirena-bg":                       {},
	"--sirena-fg":                       {},
	"--sirena-stroke":                   {},
	"--sirena-stroke-strong":            {},
	"--sirena-element-fill-service":     {},
	"--sirena-element-fill-database":    {},
	"--sirena-element-fill-queue":       {},
	"--sirena-element-fill-cache":       {},
	"--sirena-element-fill-job":         {},
	"--sirena-element-fill-external":    {},
	"--sirena-element-fill-client":      {},
	"--sirena-element-fill-gateway":     {},
	"--sirena-element-fill-node":        {},
	"--sirena-boundary-fill-trust":      {},
	"--sirena-boundary-fill-network":    {},
	"--sirena-boundary-fill-deployment": {},
	"--sirena-boundary-fill-team":       {},
	"--sirena-edge-stroke-calls":        {},
	"--sirena-edge-stroke-reads":        {},
	"--sirena-edge-stroke-writes":       {},
	"--sirena-edge-stroke-publishes":    {},
	"--sirena-edge-stroke-subscribes":   {},
	"--sirena-edge-stroke-depends-on":   {},
	"--sirena-edge-stroke-flow":         {},
	"--sirena-label-fill":               {},
	"--sirena-label-bg":                 {},
}

// Theme is a named bundle of design-token values. Tokens maps each
// allowlisted CSS-variable name to a value (a hex color in v0.1).
type Theme struct {
	Name   string
	Tokens map[string]string
}

// DefaultThemeName is the theme used when a caller asks for none.
const DefaultThemeName = "earth-default"

// ThemeForName returns the registered theme, or ErrUnknownTheme.
func ThemeForName(name string) (*Theme, error) {
	if t, ok := themes[name]; ok {
		return t, nil
	}
	return nil, ErrUnknownTheme
}

// themes is the registry. v0.1 ships one theme; its palette (olive /
// sage / rust / sand earth tones) is ratified in the layout/SVG ADR.
var themes = map[string]*Theme{
	"earth-default": {
		Name: "earth-default",
		Tokens: map[string]string{
			"--sirena-bg":                       "#f5f1e8",
			"--sirena-fg":                       "#2e2a23",
			"--sirena-stroke":                   "#8a7f6a",
			"--sirena-stroke-strong":            "#4a4334",
			"--sirena-element-fill-service":     "#c9d4b5",
			"--sirena-element-fill-database":    "#b8c79e",
			"--sirena-element-fill-queue":       "#d9c9a3",
			"--sirena-element-fill-cache":       "#e0d4b0",
			"--sirena-element-fill-job":         "#cdbfa0",
			"--sirena-element-fill-external":    "#d6cbb8",
			"--sirena-element-fill-client":      "#dcd3c0",
			"--sirena-element-fill-gateway":     "#c4d0b0",
			"--sirena-element-fill-node":        "#cdd2c0",
			"--sirena-boundary-fill-trust":      "#efe7d6",
			"--sirena-boundary-fill-network":    "#e7eede",
			"--sirena-boundary-fill-deployment": "#f0e8d4",
			"--sirena-boundary-fill-team":       "#ece4d0",
			"--sirena-edge-stroke-calls":        "#7a6f55",
			"--sirena-edge-stroke-reads":        "#5f7a52",
			"--sirena-edge-stroke-writes":       "#a35f3c",
			"--sirena-edge-stroke-publishes":    "#6a7a8a",
			"--sirena-edge-stroke-subscribes":   "#8a7a6a",
			"--sirena-edge-stroke-depends-on":   "#9a8f75",
			"--sirena-edge-stroke-flow":         "#6f6a55",
			"--sirena-label-fill":               "#2e2a23",
			"--sirena-label-bg":                 "#f5f1e8",
		},
	},
}
