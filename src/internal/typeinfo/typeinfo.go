// Package typeinfo provides type maps shared between the type checker
// and code generator, avoiding import cycles between those two packages.
package typeinfo

import (
	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/types"
)

// TypeMap maps expression AST nodes to their inferred types.
// The types are fully resolved (no free type variables) after type checking
// is complete.
type TypeMap map[ast.Expr]types.Type

// VarTypeMap maps Goop variable names to their fully resolved types.
// This is used for polymorphic prelude calls like Chan.make where the
// let-binding's scheme must be resolved to get the concrete type.
type VarTypeMap map[string]types.Type
