package lint

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_ValidGoFile(t *testing.T) {
	dir := t.TempDir()

	sdkFile := filepath.Join(dir, "workflow.openflow.go")
	content := []byte(`package main

import . "github.com/xhd2015/openflow/sdk"

func main() {
	agent := NewAgent(AgentOpts{
		Name:         "helper",
		SystemPrompt: "You are a helpful assistant.",
	})
	_, output, err := agent.Run("do something", RunOpts{})
	_ = output
	_ = err

	r := Shell("echo hello", ShellOpts{Name: "test"})
	_ = r.ExitCode
	_ = r.Feedback

	Step("step1", func() {
		Print("inside step")
	})

	Print("done")
	_ = agent
}
`)
	if err := os.WriteFile(sdkFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := Run(sdkFile, &stdout, &stderr)
	if err != nil {
		t.Fatalf("lint.Run failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
}

func TestRun_ShellWithFeedback(t *testing.T) {
	dir := t.TempDir()

	sdkFile := filepath.Join(dir, "workflow.openflow.go")
	content := []byte(`package main

import . "github.com/xhd2015/openflow/sdk"

func main() {
	r := Shell("go test ./...", ShellOpts{Name: "test"})
	if r.ExitCode != 0 {
		feedbacks := []Feedback{r.Feedback}
		_, output, _ := NewAgent(AgentOpts{
			Name:         "fixer",
			SystemPrompt: "You fix tests.",
		}).Run("fix tests", RunOpts{Feedbacks: feedbacks})
		if output != nil {
			if cmd := output.ShellSuggestions["test"]; cmd != "" {
				Shell(cmd, ShellOpts{Name: "test"})
			}
		}
	}
}
`)
	if err := os.WriteFile(sdkFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := Run(sdkFile, &stdout, &stderr)
	if err != nil {
		t.Fatalf("lint.Run failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
}

func TestRun_ShellMissingName(t *testing.T) {
	dir := t.TempDir()

	sdkFile := filepath.Join(dir, "workflow.openflow.go")
	content := []byte(`package main

import . "github.com/xhd2015/openflow/sdk"

func main() {
	NewAgent(AgentOpts{
		Name:         "coder",
		SystemPrompt: "You write code.",
	})
	Shell("echo hello", ShellOpts{})
	NewAgent(AgentOpts{
		Name:         "reviewer",
		SystemPrompt: "You review code.",
	})
}
`)
	if err := os.WriteFile(sdkFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := Run(sdkFile, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected lint to fail for unnamed shell")
	}
	if !strings.Contains(err.Error(), "[openflow lint]") {
		t.Fatalf("expected [openflow lint] in error, got: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(err.Error(), "shell missing name") {
		t.Fatalf("expected 'shell missing name' in error, got: %v", err)
	}
}

func TestRun_AgentMissingName(t *testing.T) {
	dir := t.TempDir()

	sdkFile := filepath.Join(dir, "workflow.openflow.go")
	content := []byte(`package main

import . "github.com/xhd2015/openflow/sdk"

func main() {
	agent := NewAgent(AgentOpts{
		SystemPrompt: "You are a helpful assistant.",
	})
	_, _, err := agent.Run("do something", RunOpts{})
	if err != nil {
		Print("agent error: " + err.Error())
	}
	Shell("echo hello", ShellOpts{Name: "test"})
}
`)
	if err := os.WriteFile(sdkFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := Run(sdkFile, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected lint to fail for unnamed agent")
	}
	if !strings.Contains(err.Error(), "[openflow lint]") {
		t.Fatalf("expected [openflow lint] in error, got: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(err.Error(), "agent missing name") {
		t.Fatalf("expected 'agent missing name' in error, got: %v", err)
	}
}
