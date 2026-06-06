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
	r := Shell("echo hello", ShellOpts{Name: "test"})
	if r.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d. stderr: %s", r.ExitCode, r.Stderr)
	}
	if !strings.Contains(r.Stdout, "hello") {
		t.Fatalf("expected stdout 'hello', got %q", r.Stdout)
	}
	if r.Name != "test" {
		t.Fatalf("expected Name 'test', got %q", r.Name)
	}
	if r.Feedback.Name != "test" {
		t.Fatalf("expected Feedback.Name 'test', got %q", r.Feedback.Name)
	}
	if r.Pwd == "" {
		t.Fatal("expected non-empty Pwd")
	}
	if r.Feedback.Pwd == "" {
		t.Fatal("expected non-empty Feedback.Pwd")
	}
}

func TestShellExitNonZero(t *testing.T) {
	r := Shell("exit 42", ShellOpts{Name: "fail-test"})
	if r.ExitCode != 42 {
		t.Fatalf("expected exit 42, got %d", r.ExitCode)
	}
	if r.Feedback.ExitCode != 42 {
		t.Fatalf("expected Feedback.ExitCode 42, got %d", r.Feedback.ExitCode)
	}
}

func TestShellFeedbackToAgent(t *testing.T) {
	t.Setenv("OPENFLOW_HOME", "/tmp/.openflow")
	t.Setenv("OPENFLOW_RUN_ID", "20260606-abc123")

	fb := ShellFeedback{
		Name:     "test",
		Cmd:      "go test ./...",
		Pwd:      "/home/user/project",
		ExitCode: 1,
		Stdout:   "--- FAIL: TestFoo",
		Stderr:   "exit status 1",
	}

	xml := fb.ToAgent("coder")
	if !strings.Contains(xml, `<bash name="test">`) {
		t.Fatalf("expected <bash name=\"test\">, got: %s", truncateStr(xml, 200))
	}
	if !strings.Contains(xml, `cmd: go test ./...`) {
		t.Fatalf("expected cmd line, got: %s", truncateStr(xml, 200))
	}
	if !strings.Contains(xml, `pwd: /home/user/project`) {
		t.Fatalf("expected pwd line, got: %s", truncateStr(xml, 200))
	}
	if !strings.Contains(xml, `exit_code: 1`) {
		t.Fatalf("expected exit_code line, got: %s", truncateStr(xml, 200))
	}
	if !strings.Contains(xml, "/agents/coder/shell_suggestions/test.sh") {
		t.Fatalf("expected suggestion path, got: %s", truncateStr(xml, 200))
	}
	if !strings.Contains(xml, "</bash>") {
		t.Fatalf("expected closing </bash>, got: %s", truncateStr(xml, 200))
	}
}

func TestShellFeedbackString(t *testing.T) {
	fb := ShellFeedback{
		Name:     "test",
		Cmd:      "go build",
		ExitCode: 0,
	}
	s := fb.String()
	if !strings.Contains(s, "test") {
		t.Fatalf("expected name in string, got %q", s)
	}
	if !strings.Contains(s, "go build") {
		t.Fatalf("expected cmd in string, got %q", s)
	}
}

func TestShellFeedbackToAgentTruncation(t *testing.T) {
	t.Setenv("OPENFLOW_HOME", "/tmp/.openflow")
	t.Setenv("OPENFLOW_RUN_ID", "20260606-abc123")

	bigData := strings.Repeat("abcdefghij", 200)

	fb := ShellFeedback{
		Name:     "test",
		Cmd:      "go test ./...",
		Pwd:      "/home/user/project",
		ExitCode: 1,
		Stdout:   bigData,
		Stderr:   bigData,
	}

	xml := fb.ToAgent("coder")
	if len(xml) > 1024 {
		t.Fatalf("expected XML <= 1024 chars, got %d", len(xml))
	}
	if !strings.Contains(xml, "<bash") {
		t.Fatalf("expected <bash in truncated result")
	}
}

func TestShellFeedbackToAgentNoSuggestionWhenNoEnv(t *testing.T) {
	t.Setenv("OPENFLOW_HOME", "")
	t.Setenv("OPENFLOW_RUN_ID", "")

	fb := ShellFeedback{
		Name:     "test",
		Cmd:      "go test ./...",
		Pwd:      "/home/user/project",
		ExitCode: 1,
		Stdout:   "",
		Stderr:   "",
	}

	xml := fb.ToAgent("coder")
	if strings.Contains(xml, "suggestion:") {
		t.Fatalf("expected no suggestion line when env vars missing, got: %s", xml)
	}
}

func TestShellFeedbackImplementsFeedback(t *testing.T) {
	var _ Feedback = ShellFeedback{}
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
	if S("hello") != "hello" {
		t.Fatalf("S(\"hello\") = %q", S("hello"))
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

func TestShellWithRelativeDir(t *testing.T) {
	r := Shell("echo hello", ShellOpts{Name: "rel-dir", Dir: "."})
	if r.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d. stderr: %s", r.ExitCode, r.Stderr)
	}
	if !strings.Contains(r.Stdout, "hello") {
		t.Fatalf("expected stdout 'hello', got %q", r.Stdout)
	}
	if r.Pwd == "" {
		t.Fatal("expected non-empty Pwd with relative dir")
	}
}

func TestShellNonexistentDir(t *testing.T) {
	r := Shell("echo hello", ShellOpts{Name: "bad-dir", Dir: "/nonexistent/path/12345"})
	if r.ExitCode == 0 {
		t.Fatal("expected non-zero exit for nonexistent dir")
	}
	if r.Stderr == "" {
		t.Fatal("expected stderr for nonexistent dir")
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
