// Package active tracks active pattern definitions for the Goop compiler.
//
// Active patterns let users define custom match patterns as functions
// returning an option type:
//
//	let (|Positive|_|) (n: int) : int option =
//	  if n > 0 then Some n else None
//
// A match case using an active pattern:
//
//	match x with
//	| Positive p -> expr
//	| _ -> default
//
// An active pattern is registered with its name, input type, output type,
// and the Go function name used to call it at runtime.
package active

import (
	"goop.dev/compiler/internal/types"
)

// Entry describes a registered active pattern.
type Entry struct {
	Name       string      // e.g. "Positive"
	InputType  types.Type  // the scrutinee type
	OutputType types.Type  // the wrapped value type (inside option)
	GoFuncName string      // the Go function name, e.g. "__active_Positive"
}

// Registry holds all active patterns in scope.
type Registry struct {
	entries map[string]*Entry
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]*Entry)}
}

// Register adds an active pattern.
func (r *Registry) Register(name string, inputType, outputType types.Type, goFuncName string) {
	r.entries[name] = &Entry{
		Name:       name,
		InputType:  inputType,
		OutputType: outputType,
		GoFuncName: goFuncName,
	}
}

// Lookup finds an active pattern by name.
func (r *Registry) Lookup(name string) *Entry {
	return r.entries[name]
}

// IsActivePattern reports whether a name is a registered active pattern.
func (r *Registry) IsActivePattern(name string) bool {
	_, ok := r.entries[name]
	return ok
}

// GlobalRegistry is the shared registry used during compilation.
// It is populated during type checking and read during code generation.
var GlobalRegistry = NewRegistry()
