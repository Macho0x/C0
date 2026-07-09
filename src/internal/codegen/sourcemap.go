// Package codegen — JSON source-map support.
//
// The SourceMap records mappings from Goop source locations to generated
// Go output locations.  It is serialisable as JSON for external debuggers.
package codegen

import (
	"encoding/json"
	"fmt"
	"io"
)

// SourceMap links Goop source positions to generated Go output positions.
type SourceMap struct {
	Version   int       `json:"version"`
	Source    string    `json:"source"`
	Generated string    `json:"generated"`
	Mappings  []Mapping `json:"mappings"`
}

// Mapping is a single Goop→Go position correspondence.
type Mapping struct {
	C0Line   int `json:"c0_line"`
	C0Column int `json:"c0_column"`
	GoLine   int `json:"go_line"`
	GoColumn int `json:"go_column"`
}

// NewSourceMap creates an empty source map.
func NewSourceMap(source, generated string) *SourceMap {
	return &SourceMap{
		Version:   3,
		Source:    source,
		Generated: generated,
	}
}

// Add records a single Goop→Go mapping.
func (sm *SourceMap) Add(c0Line, c0Col, goLine, goCol int) {
	sm.Mappings = append(sm.Mappings, Mapping{
		C0Line:   c0Line,
		C0Column: c0Col,
		GoLine:   goLine,
		GoColumn: goCol,
	})
}

// Write serialises the source map as indented JSON to w.
func (sm *SourceMap) Write(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sm); err != nil {
		return fmt.Errorf("encoding source map: %w", err)
	}
	return nil
}
