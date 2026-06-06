package creator

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	agentprovider "github.com/xhd2015/agent-pro/agent/cli/provider"
	"github.com/xhd2015/agent-pro/agent/cli/registry"
	agentexec "github.com/xhd2015/agent-pro/agent/exec"
	"github.com/xhd2015/openflow/internal/lint"
)

//go:embed SYSTEM_PROMPT.md
var SystemPromptTemplate string

type Options struct {
	Description  string
	OutputFile   string
	Workspace    string
	AgentRunner  string
	Model        string
	SettingsPath string
	OpenflowHome string
}

func Create(ctx context.Context, opts Options) error {
	description := strings.TrimSpace(opts.Description)
	if description == "" {
		return fmt.Errorf("description is required")
	}

	outputFile := strings.TrimSpace(opts.OutputFile)
	if outputFile == "" {
		slug := slugifyDescription(description)
		outputFile = slug + ".openflow.go"
	}

	workspace := strings.TrimSpace(opts.Workspace)
	if workspace == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workspace = wd
	}
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return err
	}

	absOutput, err := filepath.Abs(outputFile)
	if err != nil {
		return err
	}

	os.MkdirAll(filepath.Dir(absOutput), 0755)

	relOutput := absOutput
	if strings.HasPrefix(absOutput, absWorkspace+string(os.PathSeparator)) {
		relOutput = absOutput[len(absWorkspace)+1:]
	}

	runner := strings.TrimSpace(opts.AgentRunner)
	if runner == "" {
		runner = "opencode"
	}

	env := agentexec.NewEnv(&agentexec.PathsConfig{
		RootDirName: ".openflow/agent-pro",
		DataDirName: "data",
		BinDirName:  "bin",
	}, "AGENT_PRO_CONFIG_ROOT")

	provider, err := agentprovider.Build(registry.AgentRunnerID(runner), opts.SettingsPath, absWorkspace, env)
	if err != nil {
		return fmt.Errorf("build agent runner: %w", err)
	}

	fullPrompt := stripYAMLFrontmatter(SystemPromptTemplate) + "\n" + fmt.Sprintf("# Task\n%s\nWrite it to the file `%s` in the workspace.\nGenerate the file now.", description, relOutput)

	maxIterations := 3
	var sessionID string

	var lintFeedback string

	for i := 0; i < maxIterations; i++ {
		prompt := fullPrompt
		isResume := false
		if i > 0 && sessionID != "" {
			prompt = fmt.Sprintf("# Feedback\n\nThe file %s has TypeScript errors:\n\n```\n%s\n```\n\nFix the file and write it again.", relOutput, lintFeedback)
			isResume = true
		}

		var rawBuf bytes.Buffer
		askOpts := &registry.AskOptions{
			Model:     opts.Model,
			Workspace: absWorkspace,
			RawLog:    io.MultiWriter(&rawBuf),
		}
		if isResume {
			askOpts.SessionID = sessionID
		}

		fmt.Fprintf(os.Stderr, "[openflow] asking agent (attempt %d/%d)...\n", i+1, maxIterations)
		_, err := provider.Agent.Ask(ctx, prompt, askOpts, func(delta string) {
			fmt.Fprint(os.Stderr, delta)
		})
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("agent ask: %w", err)
		}

		if sessionID == "" {
			sessionID = extractSessionID(rawBuf.Bytes())
		}

		info, statErr := os.Stat(absOutput)
		if statErr != nil {
			if i+1 < maxIterations {
				lintFeedback = fmt.Sprintf("file %s not written, please retry.\n", relOutput)
				fmt.Fprintf(os.Stderr, "[openflow] file %s not written,  retrying...\n", relOutput)
				continue
			}
			return fmt.Errorf("agent did not create %s: %w", relOutput, statErr)
		}
		var lintBuf bytes.Buffer
		fmt.Fprintf(os.Stderr, "[openflow] running lint %s...\n", absOutput)
		if err := lint.Run(absOutput, &lintBuf, &lintBuf); err != nil {
			if i+1 < maxIterations {
				lintFeedback = lintBuf.String()
				fmt.Fprintf(os.Stderr, "[openflow] lint failed, retrying...\n\n%s\n\n", lintFeedback)
				continue
			}
			return fmt.Errorf("failed to create valid file after %d attempts: still has lint errors", maxIterations)
		}

		fmt.Printf("created: %s (%d bytes)\n", absOutput, info.Size())
		return nil
	}

	return fmt.Errorf("failed to create valid file after %d attempts", maxIterations)
}

func extractSessionID(rawData []byte) string {
	lines := strings.Split(string(rawData), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event struct {
			SessionID string `json:"sessionID"`
		}
		if json.Unmarshal([]byte(line), &event) == nil && event.SessionID != "" {
			return event.SessionID
		}
	}
	return ""
}

func slugifyDescription(desc string) string {
	desc = strings.ToLower(strings.TrimSpace(desc))
	var b strings.Builder
	for _, r := range desc {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' {
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteRune('-')
			}
		}
	}
	result := strings.Trim(b.String(), "-")
	if len(result) > 60 {
		result = result[:60]
	}
	if result == "" {
		return "workflow"
	}
	return result
}

func stripYAMLFrontmatter(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "---") {
		return s
	}
	rest := s[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return s
	}
	result := rest[idx+4:]
	return strings.TrimSpace(result)
}
