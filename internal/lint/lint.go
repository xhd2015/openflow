package lint

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xhd2015/openflow/internal/runner"
)

func Run(file string, stdout, stderr io.Writer) error {
	absFile, err := filepath.Abs(file)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "openflow-lint-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	baseName := filepath.Base(absFile)
	dstFile := filepath.Join(tmpDir, baseName)
	srcData, err := os.ReadFile(absFile)
	if err != nil {
		return fmt.Errorf("read workflow file: %w", err)
	}
	if err := os.WriteFile(dstFile, srcData, 0644); err != nil {
		return fmt.Errorf("copy workflow file: %w", err)
	}

	sdkModuleDir := filepath.Join(tmpDir, ".openflow-sdk")
	if err := runner.ExtractMockSDK(sdkModuleDir); err != nil {
		return fmt.Errorf("extract mock SDK: %w", err)
	}

	goModContent := fmt.Sprintf("module openflow-lint-workflow\n\ngo 1.25\n\nrequire github.com/xhd2015/openflow/sdk v0.0.0\n\nreplace github.com/xhd2015/openflow/sdk => ./%s\n", ".openflow-sdk")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		return fmt.Errorf("write go.mod: %w", err)
	}

	goBuild := exec.Command("go", "build", "-tags", "openflow", "-o", os.DevNull, ".")
	goBuild.Dir = tmpDir
	goBuild.Stdout = stdout
	goBuild.Stderr = stderr
	if err := goBuild.Run(); err != nil {
		return fmt.Errorf("lint failed")
	}

	goVet := exec.Command("go", "vet", "-tags", "openflow", ".")
	goVet.Dir = tmpDir
	goVet.Stdout = stdout
	goVet.Stderr = stderr
	if err := goVet.Run(); err != nil {
		return fmt.Errorf("lint failed")
	}

	var runStderr bytes.Buffer
	goRun := exec.Command("go", "run", "-tags", "openflow", ".")
	goRun.Dir = tmpDir
	goRun.Stdout = stdout
	goRun.Stderr = io.MultiWriter(stderr, &runStderr)
	err = goRun.Run()

	runStderrStr := runStderr.String()
	if strings.Contains(runStderrStr, "[openflow lint]") {
		return fmt.Errorf("lint validation failed:\n%s", runStderrStr)
	}

	if err != nil {
		return fmt.Errorf("lint failed")
	}

	return nil
}
