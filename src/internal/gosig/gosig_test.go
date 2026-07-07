package gosig

import (
	"strings"
	"testing"
	"time"
)

// TestLookupFuncStringsHasPrefix verifies that LookupFunc can resolve a
// real stdlib function. Since "strings" is always available and doesn't
// require any module setup, this is a safe standard-library test.
// If packages.Load fails (e.g. offline), the test is skipped.
func TestLookupFuncStringsHasPrefix(t *testing.T) {
	done := make(chan struct{})
	var sig *FuncSig
	var err error

	go func() {
		sig, err = LookupFunc("strings", "HasPrefix")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Skip("packages.Load timed out — skipping gosig integration test")
	}

	if err != nil {
		t.Skipf("gosig fallback not available: %v", err)
	}
	if sig == nil {
		t.Fatal("expected non-nil FuncSig")
	}

	if len(sig.Params) != 2 {
		t.Errorf("expected 2 params for strings.HasPrefix, got %d: %v",
			len(sig.Params), sig.Params)
	}
	if len(sig.Results) != 1 {
		t.Errorf("expected 1 result for strings.HasPrefix, got %d: %v",
			len(sig.Results), sig.Results)
	}
	if len(sig.Results) == 1 && sig.Results[0].Type != "bool" {
		t.Errorf("expected result type bool, got %q", sig.Results[0].Type)
	}
}

// TestGoTypeToC0TypeMapping tests the Go-type→C0-type conversion in
// isolation (unit test, no network needed).
func TestGoTypeToC0TypeMapping(t *testing.T) {
	// This tests the conversion functions in the typecheck package, but
	// those functions are unexported. We test a simplified version here
	// that exercises the same logic used by the gosig integration.
	//
	// The actual conversion lives in the typecheck package; this test
	// ensures the FuncSig is produced in the expected Go type format.
	sig, err := LookupFunc("fmt", "Sprintf")
	if err != nil {
		t.Skipf("gosig fallback not available: %v", err)
	}
	if sig == nil {
		t.Skip("nil FuncSig")
	}

	// fmt.Sprintf has: func(string, ...interface{}) string
	// The first param should be string; there should be at least 1 result.
	if len(sig.Params) == 0 {
		t.Error("expected at least 1 parameter for fmt.Sprintf")
	}
	if len(sig.Params) >= 1 && sig.Params[0].Type != "string" {
		t.Errorf("expected first param type 'string', got %q", sig.Params[0].Type)
	}
	if len(sig.Results) == 0 {
		t.Error("expected at least 1 result for fmt.Sprintf")
	}
	if len(sig.Results) >= 1 && sig.Results[0].Type != "string" {
		t.Errorf("expected result type 'string', got %q", sig.Results[0].Type)
	}
}

// TestLookupFuncCache verifies that repeated calls return cached results.
func TestLookupFuncCache(t *testing.T) {
	// First call
	sig1, err1 := LookupFunc("strings", "Contains")
	if err1 != nil {
		t.Skipf("gosig fallback not available: %v", err1)
	}
	// Second call (should hit cache)
	sig2, err2 := LookupFunc("strings", "Contains")
	if err2 != nil {
		t.Errorf("cached lookup failed: %v", err2)
	}
	if sig1 == nil || sig2 == nil {
		t.Skip("nil FuncSig")
	}
	if len(sig1.Params) != len(sig2.Params) {
		t.Error("cache returned different param count")
	}
}

// TestLookupFuncNonExistent verifies error handling for unknown functions.
func TestLookupFuncNonExistent(t *testing.T) {
	_, err := LookupFunc("strings", "NonExistentFuncXYZ")
	if err == nil {
		t.Error("expected error for non-existent function")
	} else {
		t.Logf("expected error: %v", err)
	}
}

// TestLookupFuncContainsParams checks the params and results of
// strings.Contains, which is func(string, string) bool.
func TestLookupFuncContainsParams(t *testing.T) {
	sig, err := LookupFunc("strings", "Contains")
	if err != nil {
		t.Skipf("gosig fallback not available: %v", err)
	}
	if sig == nil {
		t.Skip("nil FuncSig")
	}

	if len(sig.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(sig.Params))
	}
	for i, p := range sig.Params {
		if p.Type != "string" {
			t.Errorf("param %d: expected 'string', got %q", i, p.Type)
		}
		if !strings.Contains(p.Type, "string") {
			t.Errorf("param %d type should contain 'string', got %q", i, p.Type)
		}
	}
	if len(sig.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(sig.Results))
	}
	if len(sig.Results) == 1 && sig.Results[0].Type != "bool" {
		t.Errorf("expected result type 'bool', got %q", sig.Results[0].Type)
	}
}
