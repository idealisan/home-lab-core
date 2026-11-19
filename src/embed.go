// Package src embeds the frontend assets so the web panel can be built into a
// single Go binary.
package src

import "embed"

// FrontendFS contains the static frontend assets (the macOS-like desktop UI).
//
//go:embed frontend
var FrontendFS embed.FS
