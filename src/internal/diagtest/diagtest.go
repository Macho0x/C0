// Package diagtest provides helpers for compile-fail tests across safety passes.
package diagtest

import (
	"strings"
	"testing"

	"goop.dev/compiler/internal/checkpipeline"
	"goop.dev/compiler/internal/config"
	"goop.dev/compiler/internal/desugar"
	"goop.dev/compiler/internal/parser"
	"goop.dev/compiler/internal/typecheck"
)

// AssertCheckFails runs parse → typecheck → safety pipeline and expects a diagnostic code.
func AssertCheckFails(t *testing.T, source, wantCode, wantSubstr string) {
	t.Helper()
	cfg := config.DefaultConfig()
	mod, err := parser.Parse("test.goop", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mod = desugar.DesugarModule(mod)

	tm, _, typeErrs := typecheck.CheckWithTypesForFile(mod, "test.goop", cfg, nil)
	if codeInErrors(typeErrs, wantCode, wantSubstr) {
		return
	}

	linearTypes := checkpipeline.BuildLinearTypes(mod)
	checkpipeline.RegisterADTsFromModule(mod)
	r := checkpipeline.Run(mod, tm, linearTypes, cfg)

	all := append([]error{}, typeErrs...)
	all = append(all, r.LinearErrors...)
	all = append(all, r.NilchanErrors...)
	all = append(all, r.RefineErrors...)
	all = append(all, r.ExhaustErrors...)
	all = append(all, r.ExhaustWarns...)

	if !codeInErrors(all, wantCode, wantSubstr) {
		t.Fatalf("expected code %q containing %q, got errors: %v", wantCode, wantSubstr, all)
	}
}

func codeInErrors(errs []error, code, substr string) bool {
	for _, e := range errs {
		msg := e.Error()
		if code != "" && !strings.Contains(msg, code) {
			continue
		}
		if substr != "" && !strings.Contains(msg, substr) {
			continue
		}
		return true
	}
	return false
}
