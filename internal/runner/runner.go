package runner

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed sdk/*.go
var sdkFS embed.FS

type Options struct {
	OpenflowFile string
	Workspace    string
	AgentRunner  string
	Model        string
	OpenflowHome string
	OpenflowBin  string
}

type RunResult struct {
	Output   string
	Duration time.Duration
}

func Run(ctx context.Context, opts Options) (*RunResult, error) {
	openflowFile := strings.TrimSpace(opts.OpenflowFile)
	if openflowFile == "" {
		return nil, fmt.Errorf("openflow file is required")
	}
	absOpenflowFile, err := filepath.Abs(openflowFile)
	if err != nil {
		return nil, err
	}

	workspace := strings.TrimSpace(opts.Workspace)
	if workspace == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		workspace = wd
	}
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}

	openflowHome := strings.TrimSpace(opts.OpenflowHome)
	if openflowHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		openflowHome = filepath.Join(home, ".openflow")
	}

	runID := fmt.Sprintf("%s-%s", time.Now().Format("20060102"), newRunID())
	runDir := filepath.Join(openflowHome, "runs", runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, err
	}

	sdkModuleDir := filepath.Join(absWorkspace, ".openflow-sdk")
	if err := ExtractSDK(sdkModuleDir); err != nil {
		return nil, fmt.Errorf("extract SDK: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[openflow] SDK extracted to: %s\n", sdkModuleDir)

	goWorkPath, cleanup, err := PrepGoModule(absWorkspace, sdkModuleDir)
	if err != nil {
		return nil, fmt.Errorf("prepare Go module: %w", err)
	}
	defer cleanup()

	openflowBin := strings.TrimSpace(opts.OpenflowBin)
	if openflowBin == "" {
		bin, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("resolve openflow binary: %w", err)
		}
		openflowBin = bin
	}

	start := time.Now()
	fmt.Fprintf(os.Stderr, "[openflow] running: go run %s (workspace: %s)\n", absOpenflowFile, absWorkspace)
	cmd := exec.CommandContext(ctx, "go", "run", "-tags", "openflow", absOpenflowFile)
	cmd.Dir = absWorkspace
	cmd.Env = append(os.Environ(),
		"GOWORK="+goWorkPath,
		"OPENFLOW_HOME="+openflowHome,
		"OPENFLOW_RUN_ID="+runID,
		"OPENFLOW_BIN="+openflowBin,
		"OPENFLOW_WORKSPACE="+absWorkspace,
		"OPENFLOW_AGENT_RUNNER="+strings.TrimSpace(opts.AgentRunner),
		"OPENFLOW_MODEL="+strings.TrimSpace(opts.Model),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runErr := cmd.Run()

	duration := time.Since(start)

	status := "completed"
	exitCode := 0
	if runErr != nil {
		status = "failed"
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	writeRunMeta(runDir, runID, absOpenflowFile, status, exitCode, runErr, duration)

	return &RunResult{
		Output:   "[workflow output streamed above]",
		Duration: duration,
	}, runErr
}

func PrepGoModule(workspace string, sdkModuleDir string) (goWorkPath string, cleanup func(), err error) {
	noop := func() {}

	workspaceHasMod := false
	if _, err := os.Stat(filepath.Join(workspace, "go.mod")); err == nil {
		workspaceHasMod = true
	}

	if !workspaceHasMod {
		modContent := fmt.Sprintf("module openflow-workspace\n\ngo 1.25\n")
		if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte(modContent), 0644); err != nil {
			return "", noop, fmt.Errorf("write go.mod: %w", err)
		}
	}

	absSDK, err := filepath.Abs(sdkModuleDir)
	if err != nil {
		return "", noop, err
	}

	relSDK, err := filepath.Rel(workspace, absSDK)
	if err != nil {
		return "", noop, err
	}

	goWorkContent := fmt.Sprintf("go 1.25\n\nuse .\nuse ./%s\n", relSDK)
	goWorkPath = filepath.Join(workspace, "go.work.tmp")
	if err := os.WriteFile(goWorkPath, []byte(goWorkContent), 0644); err != nil {
		return "", noop, fmt.Errorf("write go.work: %w", err)
	}

	cleanup = func() {
		os.Remove(goWorkPath)
		if !workspaceHasMod {
			os.Remove(filepath.Join(workspace, "go.mod"))
		}
	}

	return goWorkPath, cleanup, nil
}

func ExtractSDK(destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	goMod := `module github.com/xhd2015/openflow/sdk

go 1.25
`
	if err := os.WriteFile(filepath.Join(destDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}

	return fs.WalkDir(sdkFS, "sdk", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		content, err := sdkFS.ReadFile(path)
		if err != nil {
			return err
		}
		relPath := strings.TrimPrefix(path, "sdk/")
		dst := filepath.Join(destDir, relPath)
		return os.WriteFile(dst, content, 0644)
	})
}

func writeRunMeta(runDir, runID, file, status string, exitCode int, runErr error, duration time.Duration) {
	meta := fmt.Sprintf(`{"id":"%s","file":"%s","status":"%s","exit_code":%d,"duration_ms":%d}`, runID, file, status, exitCode, duration.Milliseconds())
	if runErr != nil {
		meta = fmt.Sprintf(`{"id":"%s","file":"%s","status":"%s","exit_code":%d,"duration_ms":%d,"error":"%s"}`, runID, file, status, exitCode, duration.Milliseconds(), escapeJSON(runErr.Error()))
	}
	_ = os.WriteFile(filepath.Join(runDir, "metadata.json"), []byte(meta+"\n"), 0644)
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func newRunID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
