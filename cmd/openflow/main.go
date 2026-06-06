package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	agentprovider "github.com/xhd2015/agent-pro/agent/cli/provider"
	"github.com/xhd2015/agent-pro/agent/cli/registry"
	agentexec "github.com/xhd2015/agent-pro/agent/exec"
	lessflags "github.com/xhd2015/less-flags"
	"github.com/xhd2015/openflow/internal/creator"
	"github.com/xhd2015/openflow/internal/lint"
	"github.com/xhd2015/openflow/internal/runner"
	skillinstall "github.com/xhd2015/skills/install"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "openflow: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(help)
		return nil
	}

	switch args[0] {
	case "create":
		return runCreate(args[1:])
	case "run":
		return runOpenflowRun(args[1:])
	case "exec":
		return runExec(args[1:])
	case "lint":
		return runLint(args[1:])
	case "skill":
		return runSkill(args[1:])
	case "status":
		return fmt.Errorf("status not yet implemented")
	case "trace":
		return fmt.Errorf("trace not yet implemented")
	case "-h", "--help":
		fmt.Print(help)
		return nil
	default:
		return fmt.Errorf("unknown command: %s\n%s", args[0], help)
	}
}

func runCreate(args []string) error {
	var outFile string
	var dir string
	var agentRunner string
	var model string

	remain, err := lessflags.String("--out", &outFile).
		String("--dir", &dir).
		String("--agent-runner", &agentRunner).
		String("--model", &model).
		Help("-h,--help", createHelp).
		Parse(args)
	if err != nil {
		return err
	}

	description := strings.TrimSpace(strings.Join(remain, " "))
	if description == "" {
		return fmt.Errorf("description is required\n%s", createHelp)
	}

	return creator.Create(context.Background(), creator.Options{
		Description:  description,
		OutputFile:   outFile,
		Workspace:    dir,
		AgentRunner:  firstNonEmpty(agentRunner, os.Getenv("OPENFLOW_AGENT_RUNNER"), "opencode"),
		Model:        firstNonEmpty(model, os.Getenv("OPENFLOW_MODEL"), ""),
		SettingsPath: os.Getenv("OPENFLOW_SETTINGS"),
		OpenflowHome: resolveOpenflowHome(),
	})
}

func runOpenflowRun(args []string) error {
	var dir string
	var agentRunner string
	var model string

	remain, err := lessflags.String("--dir", &dir).
		String("--agent-runner", &agentRunner).
		String("--model", &model).
		Help("-h,--help", runHelp).
		Parse(args)
	if err != nil {
		return err
	}

	if len(remain) == 0 {
		return fmt.Errorf("openflow file is required\n%s", runHelp)
	}
	openflowFile := remain[0]

	result, err := runner.Run(context.Background(), runner.Options{
		OpenflowFile:  openflowFile,
		Workspace:     dir,
		AgentRunner:   firstNonEmpty(agentRunner, os.Getenv("OPENFLOW_AGENT_RUNNER"), "opencode"),
		Model:         firstNonEmpty(model, os.Getenv("OPENFLOW_MODEL"), ""),
		OpenflowHome:  resolveOpenflowHome(),
		OpenflowBin:   resolveOpenflowBin(),
	})

	if result != nil && strings.TrimSpace(result.Output) != "" {
		fmt.Print(strings.TrimRight(result.Output, "\n"))
		fmt.Println()
	}
	return err
}

func runExec(args []string) error {
	var prompt string
	var dir string
	var agentRunner string
	var model string
	var traceDir string
	var session string
	var resume bool

	remain, err := lessflags.String("--prompt", &prompt).
		String("--dir", &dir).
		String("--agent-runner", &agentRunner).
		String("--model", &model).
		String("--trace-dir", &traceDir).
		String("--session", &session).
		Bool("--resume", &resume).
		Parse(args)
	if err != nil {
		return err
	}

	_ = remain

	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fmt.Errorf("--prompt is required")
	}

	workspace := strings.TrimSpace(dir)
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

	runner := firstNonEmpty(agentRunner, os.Getenv("OPENFLOW_AGENT_RUNNER"), "opencode")

	env := agentexec.NewEnv(&agentexec.PathsConfig{
		RootDirName: ".openflow/agent-pro",
		DataDirName: "data",
		BinDirName:  "bin",
	}, "AGENT_PRO_CONFIG_ROOT")

	provider, err := agentprovider.Build(registry.AgentRunnerID(runner), os.Getenv("OPENFLOW_SETTINGS"), absWorkspace, env)
	if err != nil {
		return fmt.Errorf("build agent runner: %w", err)
	}

	var rawLog io.Writer
	var logFile *os.File
	if traceDir != "" {
		if err := os.MkdirAll(traceDir, 0755); err == nil {
			f, err := os.Create(filepath.Join(traceDir, "events.jsonl"))
			if err == nil {
				rawLog = f
				logFile = f
			}
		}
	}

	askOpts := &registry.AskOptions{
		Model:     firstNonEmpty(model, os.Getenv("OPENFLOW_MODEL"), ""),
		Workspace: absWorkspace,
		RawLog:    rawLog,
	}

	openflowHome := resolveOpenflowHome()

	if resume && session != "" {
		sessionID := readOpendcodeSessionID(openflowHome, session)
		if sessionID != "" {
			askOpts.SessionID = sessionID
		}
	}

	answer, err := provider.Agent.Ask(context.Background(), prompt, askOpts, func(delta string) {})
	if err != nil {
		if logFile != nil {
			logFile.Close()
		}
		return fmt.Errorf("agent exec: %w", err)
	}

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}

	if session != "" && !resume {
		extractedID := extractSessionIDFromTrace(traceDir)
		if extractedID != "" {
			writeSessionMapping(openflowHome, session, extractedID)
		}
	}

	if strings.TrimSpace(answer) != "" {
		fmt.Print(strings.TrimRight(answer, "\n"))
		fmt.Println()
	}
	return nil
}

func readOpendcodeSessionID(openflowHome, session string) string {
	path := filepath.Join(openflowHome, "sessions", session+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var m struct {
		OpencodeSessionID string `json:"opencode_session_id"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	return m.OpencodeSessionID
}

func writeSessionMapping(openflowHome, session, opencodeSessionID string) {
	dir := filepath.Join(openflowHome, "sessions")
	os.MkdirAll(dir, 0755)
	data, _ := json.Marshal(map[string]string{
		"opencode_session_id": opencodeSessionID,
	})
	os.WriteFile(filepath.Join(dir, session+".json"), data, 0644)
}

func extractSessionIDFromTrace(traceDir string) string {
	if traceDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(traceDir, "events.jsonl"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
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

func resolveOpenflowHome() string {
	if v := strings.TrimSpace(os.Getenv("OPENFLOW_HOME")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".openflow")
	}
	return filepath.Join(home, ".openflow")
}

func resolveOpenflowBin() string {
	if bin, err := os.Executable(); err == nil {
		return bin
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func runLint(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("file is required\n%s", lintHelp)
	}
	var failed bool
	for _, f := range args {
		if err := lint.Run(f, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "openflow: %s has errors\n", f)
			failed = true
		}
	}
	if failed {
		return fmt.Errorf("lint failed")
	}
	return nil
}

func runSkill(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected skill sub-command: show, install\n%s", skillHelp)
	}
	switch args[0] {
	case "show":
		return runSkillShow(args[1:])
	case "install":
		return runSkillInstall(args[1:])
	case "-h", "--help":
		fmt.Print(skillHelp)
		return nil
	default:
		return fmt.Errorf("unknown skill sub-command: %s\n%s", args[0], skillHelp)
	}
}

func runSkillShow(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("skill show takes no arguments\n%s", skillShowHelp)
	}
	fmt.Print(creator.SystemPromptTemplate)
	return nil
}

func runSkillInstall(args []string) error {
	return skillinstall.HandleInstall(skillinstall.InstallOptions{
		SkillDirName: "openflow",
		SkillContent: creator.SystemPromptTemplate,
		Usage:        "openflow skill install",
	}, args)
}

const help = `usage: openflow <command>

commands:
  create   generate a .openflow.go from a description
  run      execute a .openflow.go
  lint     validate a .openflow.go (Go syntax)
  exec     run a single agent task (used by SDK)
  skill    show or install the openflow skill
  status   show workflow runs
  trace    open trace viewer
`

const createHelp = `usage: openflow create <description> [options]

Generate a .openflow.go file from a natural language description.

options:
  --out FILE          output file path (default: <slug>.openflow.go)
  --dir DIR           workspace directory
  --agent-runner RUNNER  agent runner: codex, opencode, cursor (default: opencode)
  --model MODEL       model override
`

const runHelp = `usage: openflow run <file.openflow.go> [options]

Execute a .openflow.go workflow file.

options:
  --dir DIR           workspace directory
  --agent-runner RUNNER  default agent runner for workflow agents
  --model MODEL       default model for workflow agents
`

const lintHelp = `usage: openflow lint <file.openflow.go> [files...]

Validate .openflow.go files using go build and go vet.
Prints errors to stderr. Exits non-zero on build or vet errors.
`

const skillHelp = `usage: openflow skill <sub-command>

sub-commands:
  show     show the openflow skill content (SYSTEM_PROMPT.md)
  install  install the openflow skill to a directory

Run openflow skill <sub-command> -h for more details.
`

const skillShowHelp = `usage: openflow skill show

Show the content of the openflow skill (SYSTEM_PROMPT.md).
`
