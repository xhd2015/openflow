package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xhd2015/openflow/internal/creator"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	stdoutCh := make(chan []byte, 1)
	readErrCh := make(chan error, 1)
	go func() {
		data, readErr := io.ReadAll(reader)
		stdoutCh <- data
		readErrCh <- readErr
	}()

	os.Stdout = writer
	runErr := fn()
	os.Stdout = oldStdout
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	data := <-stdoutCh
	if err := <-readErrCh; err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(data), runErr
}

func TestRunSkill_NoArgs(t *testing.T) {
	err := runSkill(nil)
	if err == nil {
		t.Fatal("expected error for no args")
	}
	if !strings.Contains(err.Error(), "expected skill sub-command") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSkill_UnknownSubcommand(t *testing.T) {
	err := runSkill([]string{"unknown"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown skill sub-command") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSkillShow_NoArgs(t *testing.T) {
	output, err := captureStdout(t, func() error {
		return runSkillShow(nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != creator.SystemPromptTemplate {
		t.Fatalf("SystemPromptTemplate mismatch\nexpected:\n%s\ngot:\n%s", creator.SystemPromptTemplate, output)
	}
}

func TestRunSkillShow_ExtraArgs(t *testing.T) {
	err := runSkillShow([]string{"extra"})
	if err == nil {
		t.Fatal("expected error for extra args")
	}
	if !strings.Contains(err.Error(), "skill show takes no arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSkillInstall_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	output, err := captureStdout(t, func() error {
		return runSkillInstall([]string{"--dry-run", tmpDir})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "[dry-run]") {
		t.Fatalf("expected dry-run output, got: %s", output)
	}
	if !strings.Contains(output, "SKILL.md") {
		t.Fatalf("expected SKILL.md in output, got: %s", output)
	}

	skillFile := filepath.Join(tmpDir, "SKILL.md")
	if _, err := os.Stat(skillFile); !os.IsNotExist(err) {
		t.Fatalf("expected SKILL.md to not exist after dry-run, but it does")
	}
}

func TestRunSkillInstall_Opencode(t *testing.T) {
	tmpDir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(prevWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}

	output, err := captureStdout(t, func() error {
		return runSkillInstall([]string{"--opencode"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstdout: %s", err, output)
	}

	skillFile := filepath.Join(tmpDir, ".opencode", "skills", "openflow", "SKILL.md")
	content, readErr := os.ReadFile(skillFile)
	if readErr != nil {
		t.Fatalf("read skill file: %v\nstdout: %s", readErr, output)
	}
	if string(content) != creator.SystemPromptTemplate {
		t.Fatalf("skill content mismatch\nexpected:\n%s\ngot:\n%s", creator.SystemPromptTemplate, string(content))
	}
	if !strings.Contains(output, "Installed skill to") {
		t.Fatalf("expected 'Installed skill to' in stdout, got: %s", output)
	}
}

func TestRunSkillInstall_Default(t *testing.T) {
	tmpDir := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(prevWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}

	output, err := captureStdout(t, func() error {
		return runSkillInstall(nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstdout: %s", err, output)
	}

	skillFile := filepath.Join(tmpDir, ".agents", "skills", "openflow", "SKILL.md")
	content, readErr := os.ReadFile(skillFile)
	if readErr != nil {
		t.Fatalf("read skill file: %v\nstdout: %s", readErr, output)
	}
	if string(content) != creator.SystemPromptTemplate {
		t.Fatalf("skill content mismatch")
	}
	if !strings.Contains(output, "Installed skill to") {
		t.Fatalf("expected 'Installed skill to' in stdout, got: %s", output)
	}
}

func TestRun_DispatchSkillShow(t *testing.T) {
	output, err := captureStdout(t, func() error {
		return run([]string{"skill", "show"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != creator.SystemPromptTemplate {
		t.Fatalf("SystemPromptTemplate mismatch")
	}
}
