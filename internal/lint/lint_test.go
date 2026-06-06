package lint

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_ValidGoFile(t *testing.T) {
	dir := t.TempDir()

	sdkFile := filepath.Join(dir, "workflow.openflow.go")
	content := []byte(`//go:build openflow

package main

import . "github.com/xhd2015/openflow/sdk"

func main() {
	agent := NewAgent(AgentOpts{
		SystemPrompt: "You are a helpful assistant.",
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
