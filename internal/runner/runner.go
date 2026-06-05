package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed sdk/*
var sdkFS embed.FS

type Options struct {
	OpenflowFile   string
	Workspace      string
	AgentRunner    string
	Model          string
	OpenflowHome   string
	OpenflowBin    string
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

	// Extract SDK to workspace's node_modules/openflow/
	sdkDir := filepath.Join(absWorkspace, "node_modules", "openflow")
	if err := extractSDK(sdkDir); err != nil {
		return nil, fmt.Errorf("extract SDK: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[openflow] SDK extracted to: %s\n", sdkDir)

	// Also extract to the openflow file's directory (for import resolution)
	fileDir := filepath.Dir(absOpenflowFile)
	if fileDir != absWorkspace {
		if err := extractSDK(filepath.Join(fileDir, "node_modules", "openflow")); err != nil {
			return nil, fmt.Errorf("extract SDK to file dir: %w", err)
		}
	}

	// Resolve openflow binary path (for SDK Agent to shell out to)
	openflowBin := strings.TrimSpace(opts.OpenflowBin)
	if openflowBin == "" {
		bin, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("resolve openflow binary: %w", err)
		}
		openflowBin = bin
	}

	start := time.Now()
	fmt.Fprintf(os.Stderr, "[openflow] running: bun run %s (workspace: %s)\n", absOpenflowFile, absWorkspace)
	cmd := exec.CommandContext(ctx, "bun", "run", absOpenflowFile)
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

	// Write run metadata
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

func extractSDK(destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Write package.json for module resolution
	pkgJSON := `{"name": "openflow", "version": "0.1.0", "main": "index.ts", "types": "index.ts"}`
	if err := os.WriteFile(filepath.Join(destDir, "package.json"), []byte(pkgJSON+"\n"), 0644); err != nil {
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
