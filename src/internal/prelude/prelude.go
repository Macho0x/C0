// Package prelude defines the built-in bindings available to all Goop programs
// without explicit `open` statements.
//
// Each binding has:
//   - A user‑visible name (e.g. "print_line")
//   - A Goop type (e.g. "string -> unit")
//   - A Go lowering that describes how to emit the corresponding Go code
//
// Prelude bindings are bound into the type‑checker environment before any
// user declarations, but they are shadowable — a user can define their own
// `print_line` and the prelude version is hidden.
package prelude

import (
	"goop.dev/compiler/internal/types"
)

// Binding describes one prelude entry.
type Binding struct {
	Name     string        // user‑visible name, e.g. "print_line"
	Scheme   *types.Scheme // type scheme
	Lowering Lowering      // how to emit Go code for a call
}

// Lowering describes how a call to a prelude function is lowered to Go.
type Lowering struct {
	// Func is the Go function name (e.g. "fmt.Println").  If empty, the
	// lowering uses a custom Emit function.
	Func string
	// Pkg is the Go import path needed (e.g. "fmt", "strconv").
	Pkg string
	// Operator, if set, means the call is lowered to a binary operator.
	// For example, string_concat uses "+".
	Operator string
	// Wrap controls how arguments are passed:
	//   ""     — standard call: Func(arg1, arg2, ...)
	//   "fmt.Sprintf" — use Sprintf format string (first arg is format)
	Wrap string
	// Custom, if set, uses a built-in lowering template:
	//   "assert"       — if !arg { panic("assertion failed") }
	//   "assert_equal" — if arg1 != arg2 { panic("assert_equal failed") }
	Custom string
}

// Prelude collects all built-in bindings.
type Prelude struct {
	Bindings []Binding
	// ImportMap maps Go import paths to aliases needed by the prelude.
	ImportMap map[string]string // Go import path → alias (e.g. "fmt" → "fmt")
}

// Default returns the standard prelude.
func Default() *Prelude {
	// Fresh type variables for polymorphic bindings
	a := types.Fresh("'a")
	b := types.Fresh("'b")

	p := &Prelude{
		ImportMap: make(map[string]string),
	}

	// print_line : string -> unit
	p.add("print_line",
		types.Mono(&types.TFun{From: types.String, To: types.Unit}),
		Lowering{Func: "fmt.Println", Pkg: "fmt"},
	)

	// print : string -> unit
	p.add("print",
		types.Mono(&types.TFun{From: types.String, To: types.Unit}),
		Lowering{Func: "fmt.Print", Pkg: "fmt"},
	)

	// int_to_string : int -> string
	p.add("int_to_string",
		types.Mono(&types.TFun{From: types.Int, To: types.String}),
		Lowering{Func: "strconv.Itoa", Pkg: "strconv"},
	)

	// float_to_string : float -> string
	p.add("float_to_string",
		types.Mono(&types.TFun{From: types.Float, To: types.String}),
		Lowering{Func: "fmt.Sprintf", Pkg: "fmt", Wrap: "fmt.Sprintf"},
	)

	// string_concat : string -> string -> string
	p.add("string_concat",
		types.Mono(&types.TFun{
			From: types.String,
			To:   &types.TFun{From: types.String, To: types.String},
		}),
		Lowering{Operator: "+"},
	)

	// list_length : 'a list -> int
	listA := types.ListType(a)
	p.add("list_length",
		types.Mono(&types.TFun{From: listA, To: types.Int}),
		Lowering{Func: "len", Pkg: ""}, // built-in, no import
	)

	// list_append : 'a list -> 'a list -> 'a list
	p.add("list_append",
		types.Mono(&types.TFun{
			From: listA,
			To:   &types.TFun{From: listA, To: listA},
		}),
		Lowering{Func: "append", Pkg: ""}, // built-in, no import
	)

	// panic_message : string -> 'a
	p.add("panic_message",
		types.Mono(&types.TFun{From: types.String, To: b}),
		Lowering{Func: "panic", Pkg: ""}, // built-in, no import
	)

	// Also bind Console.print_line for backward compatibility with existing
	// examples.  This is a convenience alias that maps to the same lowering
	// as print_line.
	p.add("Console.print_line",
		types.Mono(&types.TFun{From: types.String, To: types.Unit}),
		Lowering{Func: "fmt.Println", Pkg: "fmt"},
	)

	// assert : bool -> unit
	p.add("assert",
		types.Mono(&types.TFun{From: types.Bool, To: types.Unit}),
		Lowering{Custom: "assert"},
	)

	// assert_equal : 'a -> 'a -> unit  (polymorphic equality for basic types)
	p.add("assert_equal",
		&types.Scheme{
			Vars: []*types.TVar{a},
			Type: &types.TFun{
				From: a,
				To:   &types.TFun{From: a, To: types.Unit},
			},
		},
		Lowering{Custom: "assert_equal"},
	)

	// chanMake: unit -> 'a chan
	p.add("Chan.make",
		&types.Scheme{
			Vars: []*types.TVar{a},
			Type: &types.TFun{From: types.Unit, To: &types.TChan{Elem: a}},
		},
		Lowering{Custom: "chan_make"},
	)
	// chanSend: 'a chan -> 'a -> unit
	p.add("Chan.send",
		&types.Scheme{
			Vars: []*types.TVar{a},
			Type: &types.TFun{
				From: &types.TChan{Elem: a},
				To:   &types.TFun{From: a, To: types.Unit},
			},
		},
		Lowering{Custom: "chan_send"},
	)
	// chanRecv: 'a chan -> 'a
	p.add("Chan.recv",
		&types.Scheme{
			Vars: []*types.TVar{a},
			Type: &types.TFun{From: &types.TChan{Elem: a}, To: a},
		},
		Lowering{Custom: "chan_recv"},
	)
	// chanClose: 'a chan -> unit
	p.add("Chan.close",
		&types.Scheme{
			Vars: []*types.TVar{a},
			Type: &types.TFun{From: &types.TChan{Elem: a}, To: types.Unit},
		},
		Lowering{Custom: "chan_close"},
	)

	// OwnedChan.make: unit -> 'a owned_chan
	p.add("OwnedChan.make",
		&types.Scheme{
			Vars: []*types.TVar{a},
			Type: &types.TFun{From: types.Unit, To: &types.TAdt{Name: "owned_chan", Params: []types.Type{a}}},
		},
		Lowering{Custom: "owned_chan_make"},
	)

	// OwnedChan.send: 'a owned_chan -> 'a -> unit
	p.add("OwnedChan.send",
		&types.Scheme{
			Vars: []*types.TVar{a},
			Type: &types.TFun{
				From: &types.TAdt{Name: "owned_chan", Params: []types.Type{a}},
				To:   &types.TFun{From: a, To: types.Unit},
			},
		},
		Lowering{Custom: "owned_chan_send"},
	)

	// OwnedChan.recv: 'a owned_chan -> 'a
	p.add("OwnedChan.recv",
		&types.Scheme{
			Vars: []*types.TVar{a},
			Type: &types.TFun{
				From: &types.TAdt{Name: "owned_chan", Params: []types.Type{a}},
				To:   a,
			},
		},
		Lowering{Custom: "owned_chan_recv"},
	)

	// OwnedChan.close: 'a owned_chan -> unit
	p.add("OwnedChan.close",
		&types.Scheme{
			Vars: []*types.TVar{a},
			Type: &types.TFun{
				From: &types.TAdt{Name: "owned_chan", Params: []types.Type{a}},
				To:   types.Unit,
			},
		},
		Lowering{Custom: "owned_chan_close"},
	)

	// http_get_string : string -> string
	p.add("http_get_string",
		types.Mono(&types.TFun{From: types.String, To: types.String}),
		Lowering{Custom: "http_get_string"},
	)

	// json_extract_floats : string -> int -> float list
	p.add("json_extract_floats",
		types.Mono(&types.TFun{
			From: types.String,
			To:   &types.TFun{From: types.Int, To: types.ListType(types.Float)},
		}),
		Lowering{Custom: "json_extract_floats"},
	)

	// json_extract_strings : string -> int -> string list
	p.add("json_extract_strings",
		types.Mono(&types.TFun{
			From: types.String,
			To:   &types.TFun{From: types.Int, To: types.ListType(types.String)},
		}),
		Lowering{Custom: "json_extract_strings"},
	)

	_ = a
	_ = b
	return p
}

func (p *Prelude) add(name string, scheme *types.Scheme, lower Lowering) {
	p.Bindings = append(p.Bindings, Binding{
		Name:     name,
		Scheme:   scheme,
		Lowering: lower,
	})
	if lower.Pkg != "" {
		p.ImportMap[lower.Pkg] = lower.Pkg
	}
}

// Lookup finds a prelude binding by name, or returns nil.
func (p *Prelude) Lookup(name string) *Binding {
	for i := range p.Bindings {
		if p.Bindings[i].Name == name {
			return &p.Bindings[i]
		}
	}
	return nil
}
