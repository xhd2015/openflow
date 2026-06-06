package sdk

import (
	"strings"
	"testing"
)

func TestDeriveAgentName(t *testing.T) {
	name := deriveAgentName("You are a Go programmer. Fix bugs carefully.")
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		t.Fatalf("expected at least 2 parts in agent name, got %q", name)
	}
	hashPart := parts[len(parts)-1]
	if len(hashPart) != 8 {
		t.Fatalf("expected 8-char hash suffix, got %q", hashPart)
	}
}

func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()
	if !strings.Contains(id1, "openflow_session_") {
		t.Fatalf("expected session ID prefix, got %q", id1)
	}
	if id1 == id2 {
		t.Fatalf("expected unique session IDs, got both %q", id1)
	}
}

func TestShellExitZero(t *testing.T) {
	r := Shell("echo hello")
	if r.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d. stderr: %s", r.ExitCode, r.Stderr)
	}
	if !strings.Contains(r.Stdout, "hello") {
		t.Fatalf("expected stdout 'hello', got %q", r.Stdout)
	}
	if r.Feedback == "" {
		t.Fatal("expected non-empty feedback")
	}
}

func TestShellExitNonZero(t *testing.T) {
	r := Shell("exit 42")
	if r.ExitCode != 42 {
		t.Fatalf("expected exit 42, got %d", r.ExitCode)
	}
}

func TestTruncateStr(t *testing.T) {
	s := "this is a very long string that needs truncation"
	result := truncateStr(s, 10)
	if len(result) > 10 {
		t.Fatalf("expected truncated to <= 10, got %q (len=%d)", result, len(result))
	}
}

func TestPrint(t *testing.T) {
	Print("hello")
}

func TestS(t *testing.T) {
	if S(42) != "42" {
		t.Fatalf("S(42) = %q", S(42))
	}
}

func TestQuoteArg(t *testing.T) {
	if quoteArg("hello") != "hello" {
		t.Fatalf("expected unquoted 'hello', got %q", quoteArg("hello"))
	}
	if quoteArg("hello world") != "'hello world'" {
		t.Fatalf("expected quoted 'hello world', got %q", quoteArg("hello world"))
	}
}

func TestStep(t *testing.T) {
	called := false
	Step("test-step", func() {
		called = true
	})
	if !called {
		t.Fatal("Step did not call the function")
	}
}
