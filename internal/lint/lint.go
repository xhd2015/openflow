package lint

import (
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
	fileDir := filepath.Dir(absFile)

	sdkModuleDir := filepath.Join(fileDir, ".openflow-sdk")
	if err := runner.ExtractSDK(sdkModuleDir); err != nil {
		return fmt.Errorf("extract SDK: %w", err)
	}

	goWorkPath, cleanup, err := runner.PrepGoModule(fileDir, sdkModuleDir)
	if err != nil {
		return fmt.Errorf("prepare Go module: %w", err)
	}
	defer cleanup()

	baseName := strings.TrimSuffix(filepath.Base(absFile), ".go") + ".go"

	goBuild := exec.Command("go", "build", "-tags", "openflow", "-o", os.DevNull, baseName)
	goBuild.Dir = fileDir
	goBuild.Env = append(os.Environ(), "GOWORK="+goWorkPath)
	goBuild.Stdout = stdout
	goBuild.Stderr = stderr
	if err := goBuild.Run(); err != nil {
		return fmt.Errorf("lint failed")
	}

	goVet := exec.Command("go", "vet", "-tags", "openflow", baseName)
	goVet.Dir = fileDir
	goVet.Env = append(os.Environ(), "GOWORK="+goWorkPath)
	goVet.Stdout = stdout
	goVet.Stderr = stderr
	if err := goVet.Run(); err != nil {
		return fmt.Errorf("lint failed")
	}

	return nil
}
