// Package config reads the project-level c0.toml configuration file and
// resolves C0 module names to Go import paths.
//
// Configuration format (c0.toml):
//
//	module_root = "github.com/example/project"
//
//	[mappings]
//	"Std.IO"   = "c0.dev/std/io"
//	"Std.List" = "c0.dev/std/list"
//
// When no config file exists, built-in defaults are used:
//
//	Std.IO   → c0.dev/std/io   (package io)
//	Std.List → c0.dev/std/list (package list)
//
// Project modules without an explicit mapping are resolved by combining
// ModuleRoot with the lowercased, slash-separated module segments.
//
//	Example: "Trading.OrderBook" with root "github.com/user/p"
//	         → "github.com/user/p/trading/orderbook", package "orderbook"
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds the project-wide compiler configuration.
type Config struct {
	ModuleRoot string            // e.g. "github.com/example/project"
	Mappings   map[string]string // C0 module name → Go import path
}

// DefaultConfig returns a working config with built-in mappings.
func DefaultConfig() *Config {
	return &Config{
		ModuleRoot: "",
		Mappings: map[string]string{
			"Std.IO":   "c0.dev/std/io",
			"Std.List": "c0.dev/std/list",
		},
	}
}

// LoadConfig reads a c0.toml file from the given path.
// If the file does not exist, DefaultConfig is returned.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	return parseConfig(string(data))
}

// ResolveImport resolves a C0 module name (e.g. "Std.IO", "Trading.OrderBook")
// to a Go import path and package name.
func (c *Config) ResolveImport(c0ModuleName string) (goImportPath, goPackageName string) {
	// 1. Check explicit mappings
	if path, ok := c.Mappings[c0ModuleName]; ok {
		return path, packageNameFromPath(path)
	}

	// 2. Built-in defaults
	switch c0ModuleName {
	case "Std.IO":
		return "c0.dev/std/io", "io"
	case "Std.List":
		return "c0.dev/std/list", "list"
	}

	// 3. Project module: combine ModuleRoot with lowercased path segments
	segments := strings.Split(c0ModuleName, ".")
	for i, s := range segments {
		segments[i] = strings.ToLower(s)
	}
	pkg := segments[len(segments)-1]

	if c.ModuleRoot != "" {
		return c.ModuleRoot + "/" + strings.Join(segments, "/"), pkg
	}

	// 4. Fallback: treat the module name as a Go import path
	return strings.ToLower(c0ModuleName), pkg
}

// packageNameFromPath extracts the last segment of a Go import path.
func packageNameFromPath(path string) string {
	segments := strings.Split(path, "/")
	return segments[len(segments)-1]
}

// ---------------------------------------------------------------------------
// Minimal TOML parser (handles only the subset needed for c0.toml)
// ---------------------------------------------------------------------------

func parseConfig(data string) (*Config, error) {
	c := DefaultConfig()
	c.Mappings = make(map[string]string) // start fresh, copy defaults if needed

	lines := strings.Split(data, "\n")
	inSection := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := line[1 : len(line)-1]
			inSection = (section == "mappings")
			continue
		}

		// Key = value
		if idx := strings.IndexByte(line, '='); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			// Strip surrounding quotes from both key and value
			key = strings.Trim(key, `"`)
			key = strings.Trim(key, `'`)
			val = strings.Trim(val, `"`)
			val = strings.Trim(val, `'`)

			if inSection {
				c.Mappings[key] = val
			} else if key == "module_root" {
				c.ModuleRoot = val
			}
		}
	}

	// Merge built-in defaults for any mappings not explicitly overridden
	defaults := DefaultConfig()
	for k, v := range defaults.Mappings {
		if _, ok := c.Mappings[k]; !ok {
			c.Mappings[k] = v
		}
	}

	return c, nil
}
