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

//go:embed sdk_mock/*.go
var sdkMockFS embed.FS

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

	moduleDir, err := os.MkdirTemp("", "openflow-run-*")
	if err != nil {
		return nil, fmt.Errorf("create temp module dir: %w", err)
	}
	defer os.RemoveAll(moduleDir)

	sdkDir := filepath.Join(moduleDir, "openflow-sdk")
	if err := ExtractSDK(sdkDir); err != nil {
		return nil, fmt.Errorf("extract SDK: %w", err)
	}

	workflowSrc, err := os.ReadFile(absOpenflowFile)
	if err != nil {
		return nil, fmt.Errorf("read workflow file: %w", err)
	}
	workflowSrc = stripBuildTags(workflowSrc)
	workflowDst := filepath.Join(moduleDir, "workflow.go")
	if err := os.WriteFile(workflowDst, workflowSrc, 0644); err != nil {
		return nil, fmt.Errorf("write workflow file: %w", err)
	}

	goMod := `module openflow-run

go 1.25

require github.com/xhd2015/openflow/sdk v0.0.0

replace github.com/xhd2015/openflow/sdk => ./openflow-sdk
`
	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return nil, fmt.Errorf("write go.mod: %w", err)
	}

	openflowBin := strings.TrimSpace(opts.OpenflowBin)
	if openflowBin == "" {
		bin, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("resolve openflow binary: %w", err)
		}
		openflowBin = bin
	}

	binaryPath := filepath.Join(moduleDir, "openflow-run")
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = moduleDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("build workflow: %s\n%s", err, out)
	}

	start := time.Now()
	fmt.Fprintf(os.Stderr, "[openflow] running: %s (workspace: %s)\n", absOpenflowFile, absWorkspace)

	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Dir = absWorkspace
	cmd.Env = append(os.Environ(),
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

func stripBuildTags(src []byte) []byte {
	lines := strings.Split(string(src), "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//go:build openflow") || strings.HasPrefix(trimmed, "// +build openflow") {
			continue
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
}

func ExtractSDK(destDir string) error {
	return extractSDKFromFS(sdkFS, "sdk", destDir)
}

func ExtractMockSDK(destDir string) error {
	return extractSDKFromFS(sdkMockFS, "sdk_mock", destDir)
}

func extractSDKFromFS(src embed.FS, prefix string, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	goMod := `module github.com/xhd2015/openflow/sdk

go 1.25
`
	if err := os.WriteFile(filepath.Join(destDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}

	return fs.WalkDir(src, prefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		content, err := src.ReadFile(path)
		if err != nil {
			return err
		}
		relPath := strings.TrimPrefix(path, prefix+"/")
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
