// Package deadlock detects narrow static channel deadlock patterns.
package deadlock

import (
	"fmt"

	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/config"
	"goop.dev/compiler/internal/token"
)

// Error is a potential deadlock warning.
type Error struct {
	Code string
	Msg  string
	Loc  token.SourceLoc
}

func (e *Error) Error() string {
	prefix := e.Code + ": "
	if e.Loc.File != "" && e.Loc.Line > 0 {
		return fmt.Sprintf("%s:%d:%d: %s%s", e.Loc.File, e.Loc.Line, e.Loc.Column, prefix, e.Msg)
	}
	return prefix + e.Msg
}

func (e *Error) GetLoc() token.SourceLoc { return e.Loc }

type eventKind int

const (
	evSend eventKind = iota
	evRecv
)

type commEvent struct {
	Kind    eventKind
	Channel string
	Loc     token.SourceLoc
}

type goroutineBody struct {
	Site   token.SourceLoc
	Events []commEvent
}

// CheckWithConfig runs narrow static deadlock detection.
func CheckWithConfig(mod *ast.Module, cfg *config.Config) (errors, warnings []error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	c := &checker{cfg: cfg}
	for _, d := range mod.Decls {
		if ld, ok := d.(*ast.LetDecl); ok {
			for i := range ld.Bindings {
				c.collectFromExpr(ld.Bindings[i].Body)
			}
		}
	}
	c.detectCycles()
	return c.errors, c.warnings
}

type checker struct {
	errors   []error
	warnings []error
	bodies   []goroutineBody
	cfg      *config.Config
}

func (c *checker) warn(loc token.SourceLoc, msg string) {
	e := &Error{Code: "DEADLOCK001", Msg: msg, Loc: loc}
	switch c.cfg.Check.Deadlock {
	case config.SeverityOff:
		return
	case config.SeverityError:
		c.errors = append(c.errors, e)
	default:
		c.warnings = append(c.warnings, e)
	}
}

func (c *checker) collectFromExpr(e ast.Expr) {
	switch e := e.(type) {
	case *ast.GoExpr:
		body := extractStraightLine(e.Expr)
		if len(body.Events) > 0 {
			c.bodies = append(c.bodies, body)
		}
	case *ast.LetInExpr:
		for i := range e.Bindings {
			c.collectFromExpr(e.Bindings[i].Body)
		}
		c.collectFromExpr(e.Body)
	case *ast.IfExpr:
		c.collectFromExpr(e.ThenBranch)
		c.collectFromExpr(e.ElseBranch)
	case *ast.AppExpr:
		c.collectFromExpr(e.Func)
		c.collectFromExpr(e.Arg)
	case *ast.FunExpr:
		c.collectFromExpr(e.Body)
	case *ast.ParenExpr:
		c.collectFromExpr(e.Inner)
	}
}

func extractStraightLine(e ast.Expr) goroutineBody {
	var body goroutineBody
	fun := unwrapParen(e)
	fe, ok := fun.(*ast.FunExpr)
	if !ok {
		return body
	}
	body.Site = fe.Loc
	body.Events = straightLineEvents(fe.Body)
	return body
}

func unwrapParen(e ast.Expr) ast.Expr {
	for {
		if p, ok := e.(*ast.ParenExpr); ok {
			e = p.Inner
			continue
		}
		break
	}
	return e
}

func straightLineEvents(e ast.Expr) []commEvent {
	var events []commEvent
	for {
		switch v := e.(type) {
		case *ast.ParenExpr:
			e = v.Inner
			continue
		case *ast.AppExpr:
			if ev, ok := commFromApp(v); ok {
				events = append(events, ev)
				// sequential: look for next op in let-in chain
				e = nextSequential(v)
				if e == nil {
					return events
				}
				continue
			}
			return events
		case *ast.LetInExpr:
			if ev := eventsFromLetIn(v); len(ev) > 0 {
				events = append(events, ev...)
			}
			return events
		default:
			return events
		}
	}
}

func eventsFromLetIn(e *ast.LetInExpr) []commEvent {
	var events []commEvent
	for _, b := range e.Bindings {
		if ev, ok := commFromExpr(b.Body); ok {
			events = append(events, ev)
		} else {
			return nil // non-straight-line
		}
	}
	if e.Body != nil {
		events = append(events, straightLineEvents(e.Body)...)
	}
	return events
}

func nextSequential(after *ast.AppExpr) ast.Expr {
	// body may be let _ = Chan.send ... in next
	return nil
}

func commFromExpr(e ast.Expr) (commEvent, bool) {
	if app, ok := e.(*ast.AppExpr); ok {
		return commFromApp(app)
	}
	return commEvent{}, false
}

func commFromApp(app *ast.AppExpr) (commEvent, bool) {
	if send, ch, loc := parseChanSend(app); send {
		return commEvent{Kind: evSend, Channel: ch, Loc: loc}, true
	}
	if recv, ch, loc := parseChanRecv(app); recv {
		return commEvent{Kind: evRecv, Channel: ch, Loc: loc}, true
	}
	return commEvent{}, false
}

func (c *checker) detectCycles() {
	if len(c.bodies) < 2 {
		return
	}
	// Classic 2-goroutine cycle: G1 send ch1, recv ch2; G2 send ch2, recv ch1
	for i := 0; i < len(c.bodies); i++ {
		for j := i + 1; j < len(c.bodies); j++ {
			if cyclicPair(c.bodies[i].Events, c.bodies[j].Events) {
				loc := c.bodies[i].Site
				c.warn(loc, "potential channel deadlock between goroutines (circular send/recv on unbuffered channels)")
			}
		}
	}
}

func cyclicPair(a, b []commEvent) bool {
	if len(a) < 2 || len(b) < 2 {
		return false
	}
	// first send then recv pattern
	if a[0].Kind != evSend || a[1].Kind != evRecv || b[0].Kind != evSend || b[1].Kind != evRecv {
		return false
	}
	return a[0].Channel == b[1].Channel && a[1].Channel == b[0].Channel &&
		a[0].Channel != a[1].Channel
}
