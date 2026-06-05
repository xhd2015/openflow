package creator

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agentprovider "github.com/xhd2015/agent-pro/agent/cli/provider"
	"github.com/xhd2015/agent-pro/agent/cli/registry"
	agentexec "github.com/xhd2015/agent-pro/agent/exec"
)

//go:embed SYSTEM_PROMPT.md
var systemPromptTemplate string

type Options struct {
	Description   string
	OutputFile    string
	Workspace     string
	AgentRunner   string
	Model         string
	SettingsPath  string
	OpenflowHome  string
}

func Create(ctx context.Context, opts Options) error {
	description := strings.TrimSpace(opts.Description)
	if description == "" {
		return fmt.Errorf("description is required")
	}

	outputFile := strings.TrimSpace(opts.OutputFile)
	if outputFile == "" {
		slug := slugifyDescription(description)
		outputFile = slug + ".openflow.ts"
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

	prompt := strings.Replace(systemPromptTemplate, "__DESCRIPTION__", description, 1)

	answer, err := provider.Agent.Ask(ctx, prompt, &registry.AskOptions{
		Model:     opts.Model,
		Workspace: absWorkspace,
	}, func(delta string) {})
	if err != nil {
		return fmt.Errorf("agent ask: %w", err)
	}

	content := cleanGeneratedCode(answer)
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("agent returned empty response")
	}

	absOutput, err := filepath.Abs(outputFile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absOutput), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(absOutput, []byte(content+"\n"), 0644); err != nil {
		return err
	}

	fmt.Printf("created: %s (%d bytes)\n", absOutput, len(content))
	return nil
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

func cleanGeneratedCode(answer string) string {
	answer = strings.TrimSpace(answer)
	if strings.HasPrefix(answer, "```") {
		idx := strings.Index(answer, "\n")
		if idx >= 0 {
			answer = answer[idx+1:]
		}
	}
	if strings.HasSuffix(answer, "```") {
		answer = strings.TrimSuffix(answer, "```")
		answer = strings.TrimSpace(answer)
	}
	return answer
}
