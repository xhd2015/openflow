package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_HandlesDotTxtFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "openflow-runner-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	workflowContent := `package main

import "github.com/xhd2015/openflow/sdk"

func main() {
	sdk.Step("test", func() {})
}
`
	workflowFile := filepath.Join(dir, "test-workflow.openflow.go.txt")
	if err := os.WriteFile(workflowFile, []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	openflowHome := filepath.Join(dir, ".openflow")
	if err := os.MkdirAll(openflowHome, 0755); err != nil {
		t.Fatal(err)
	}

	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	result, runErr := Run(context.Background(), Options{
		OpenflowFile: workflowFile,
		Workspace:    dir,
		OpenflowHome: openflowHome,
		OpenflowBin:  bin,
	})
	if runErr != nil {
		t.Fatalf("Run() with .txt file failed: %v", runErr)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}

	sdkDir := filepath.Join(dir, ".openflow-sdk")
	if _, err := os.Stat(sdkDir); err == nil {
		t.Error("expected no .openflow-sdk in workspace")
	}

	goWorkFile := filepath.Join(dir, "go.work.tmp")
	if _, err := os.Stat(goWorkFile); err == nil {
		t.Error("expected no go.work.tmp in workspace")
	}
}

func TestRun_RejectsEmptyFile(t *testing.T) {
	_, err := Run(context.Background(), Options{})
	if err == nil {
		t.Fatal("expected error for empty file")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error, got: %v", err)
	}
}

func TestRun_NormalGoFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "openflow-runner-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	workflowContent := `package main

import "github.com/xhd2015/openflow/sdk"

func main() {
	sdk.Step("test", func() {})
}
`
	workflowFile := filepath.Join(dir, "test-workflow.openflow.go")
	if err := os.WriteFile(workflowFile, []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	openflowHome := filepath.Join(dir, ".openflow")
	if err := os.MkdirAll(openflowHome, 0755); err != nil {
		t.Fatal(err)
	}

	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	result, runErr := Run(context.Background(), Options{
		OpenflowFile: workflowFile,
		Workspace:    dir,
		OpenflowHome: openflowHome,
		OpenflowBin:  bin,
	})
	if runErr != nil {
		t.Fatalf("Run() with .go file failed: %v", runErr)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	sdkDir := filepath.Join(dir, ".openflow-sdk")
	if _, err := os.Stat(sdkDir); err == nil {
		t.Error("expected no .openflow-sdk in workspace")
	}
}
