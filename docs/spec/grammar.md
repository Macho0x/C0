# Goop Grammar

This is a high-level grammar in EBNF-like notation. It is not a complete LALR(1) grammar; it describes the intended surface syntax for specification purposes.

## Lexical elements

```
ident    := [a-zA-Z_][a-zA-Z0-9_']*
tyvar    := '\'' ident
constr   := [A-Z][a-zA-Z0-9_']*
integer  := [0-9]+
float    := [0-9]+ '.' [0-9]* | [0-9]* '.' [0-9]+
string   := '"' [^"]* '"'
char     := '\'' [^\']* '\''
unit     := '()'
```

## Reserved words

```
and as begin chan do done end for go goop else false fun golang guard if import in
let match module move mutable of panic private rec region result
requires returns select then to true type unit val when with async
```

## Program structure

```
program      := module_decl import_decl* top_decl*
module_decl  := 'module' constr ('.' constr)*
import_decl  := 'import' import_spec | 'import' '(' import_spec* ')'
import_spec  := ident? ('golang' | 'c0') '.'? string import_vals?
import_vals  := '{' 'val' ident ':' type* '}'
top_decl     := val_decl | type_decl | golang_embed_decl
golang_embed_decl := '@golang' '{' raw_go_code '}'
```

## Value declarations

```
val_decl     := 'let' rec? 'mutable'? binding ('and' binding)*
binding      := ident param* (':' type)? '=' expr
param        := ident | '(' ident ':' type ')'

expr         := if_expr
              | match_expr
              | let_expr
              | fun_expr
              | guard_expr
              | binary_expr

if_expr      := 'if' expr 'then' expr 'else' expr
match_expr   := 'match' expr 'with' '|'? case ('|' case)*
case         := pattern ('when' expr)? '->' expr
let_expr     := 'let' binding ('and' binding)* 'in' expr
fun_expr     := 'fun' param+ '->' expr
guard_expr   := 'guard' pattern '=' expr 'else' expr

binary_expr  := pipeline_expr
pipeline_expr:= app_expr ('|>' app_expr)*
app_expr     := primary_expr (primary_expr | '?' qarg?)*
primary_expr := literal
              | ident
              | constr
              | qualified_constr
              | '(' expr ')'
              | record_expr
              | list_expr
              | tuple_expr
              | field_expr
              | index_expr
              | match_macro_expr
              | go_expr
              | for_expr
              | begin_expr

for_expr     := 'for' ident '=' expr 'to' expr 'do' expr 'done'
begin_expr   := 'begin' expr (';' expr)* 'end'
index_expr   := expr '.(' expr ')'
assign_expr  := expr '<-' expr
qualified_constr := constr '.' constr

go_expr      := 'go' ('(' 'move' ident (',' ident)* ')')? expr

qarg         := string | expr

literal      := integer | float | string | char | 'true' | 'false' | '()'
record_expr  := '{' field_init (';' field_init)* '}'
field_init   := ident '=' expr | ident
list_expr    := '[' (expr (';' expr)*)? ']'
tuple_expr   := '(' expr (',' expr)+ ')'
field_expr   := expr '.' ident

match_macro_expr := expr 'is' pattern
                  | expr 'as' pattern '->' expr 'else' expr
```

## Patterns

```
pattern      := constr_pattern
              | record_pattern
              | tuple_pattern
              | list_pattern
              | literal_pattern
              | ident_pattern
              | wildcard_pattern
              | alias_pattern

constr_pattern := constr ('of' pattern)?
record_pattern := '{' field_pattern (';' field_pattern)* ('|' '..'' )? '}'
field_pattern  := ident ('=' pattern)?
tuple_pattern  := '(' pattern (',' pattern)+ ')'
list_pattern   := '[' (pattern (';' pattern)*)? ']'
alias_pattern  := pattern 'as' ident
wildcard_pattern := '_'
ident_pattern  := ident
literal_pattern:= literal
```

## Type expressions

```
type         := fn_type
fn_type      := tuple_type ('->' fn_type)? effect_row?
effect_row   := 'with' '{' effect_list '}'
effect_list  := effect (';' effect)* ('|' '..')?
              | (* empty *)
effect       := ident

tuple_type   := app_type ('*' app_type)*
app_type     := primary_type (primary_type)*
primary_type := ident
              | tyvar
              | '(' type ')'
              | type 'array'
              | '{' field_type (';' field_type)* ('|' '..')? '}'
              | '(' type (',' type)+ ')'

field_type   := ident ':' type

(* Refinement contracts *)
refined_type := type 'where' expr
```

## Type declarations

```
type_decl    := 'type' type_binding ('and' type_binding)*
type_binding := ident type_param* linear? '=' type_rhs
              | ident type_param* linear
linear       := ':' '1'
type_param   := tyvar
type_rhs     := record_type
              | adt_type
              | type

record_type  := '{' field_type (';' field_type)* '}'
adt_type     := '|'? adt_case ('|' adt_case)*
adt_case     := constr ('of' type)?
```

## Computation expressions

```
comp_expr    := builder '{' comp_ops '}'
builder      := 'result' | 'region' | 'async'

comp_ops     := (comp_op)* (comp_return | expr)
comp_op      := 'let!' pattern '=' expr
              | 'do!' expr
              | 'let' binding
              | 'return!' expr
comp_return  := 'return' expr
```

Legacy `extern "go"` and `open` declarations are parse errors (removed in v0.3). Use `import golang` / `import goop` and `@golang { ... }` embed blocks instead. See [05-modules-and-packages.md](../design/05-modules-and-packages.md).
