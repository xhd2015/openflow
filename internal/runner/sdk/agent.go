package sdk

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

type AgentOpts struct {
	Name         string
	SystemPrompt string
	AgentRunner  string
	Model        string
}

type Agent struct {
	name         string
	systemPrompt string
	agentRunner  string
	model        string
	sessionID    string
	isFirstRun   bool
	lastTask     string
}

func NewAgent(opts AgentOpts) *Agent {
	runner := opts.AgentRunner
	if runner == "" {
		runner = envOr("OPENFLOW_AGENT_RUNNER", "opencode")
	}
	model := opts.Model
	if model == "" {
		model = os.Getenv("OPENFLOW_MODEL")
	}
	name := opts.Name
	if name == "" {
		name = deriveAgentName(opts.SystemPrompt)
	}
	return &Agent{
		name:         name,
		systemPrompt: opts.SystemPrompt,
		agentRunner:  runner,
		model:        model,
		sessionID:    generateSessionID(),
		isFirstRun:   true,
	}
}

type RunOpts struct {
	Feedback string
}

func (a *Agent) Run(task string, opts RunOpts) (string, error) {
	feedback := opts.Feedback
	isResume := !a.isFirstRun && feedback != "" && task == a.lastTask
	a.lastTask = task

	var prompt string
	if isResume {
		prompt = "# Feedback\n" + feedback
		logMsg("agent", a.name+" resume: \""+truncateStr(feedback, 100)+"\"")
	} else {
		prompt = a.systemPrompt + "\n\n# Task\n" + task
		logMsg("agent", a.name+" run: \""+truncateStr(task, 100)+"\"")
	}

	startMs := time.Now()

	emitEvent(Event{
		Type:      "agent",
		Agent:     a.name,
		Status:    "started",
		Prompt:    task,
		Resume:    isResume,
		Timestamp: time.Now().Format(time.RFC3339),
	})

	args := []string{
		"exec",
		"--prompt", prompt,
		"--agent-runner", a.agentRunner,
		"--session", a.sessionID,
	}
	if a.model != "" {
		args = append(args, "--model", a.model)
	}
	if isResume {
		args = append(args, "--resume")
	}
	dir := envOr("OPENFLOW_WORKSPACE", "")
	if dir == "" {
		dir, _ = os.Getwd()
	}
	args = append(args, "--dir", dir)

	runID := os.Getenv("OPENFLOW_RUN_ID")
	if runID != "" {
		home := envOr("OPENFLOW_HOME", "")
		if home == "" {
			home, _ = os.UserHomeDir()
			home += "/.openflow"
		}
		args = append(args, "--trace-dir", home+"/runs/"+runID+"/agents/"+a.name)
	}

	openflowBin := envOr("OPENFLOW_BIN", "openflow")
	cmdStr := openflowBin
	for _, arg := range args {
		cmdStr += " " + quoteArg(arg)
	}

	result := Shell(cmdStr)
	durationMs := time.Since(startMs).Milliseconds()

	if a.isFirstRun {
		a.isFirstRun = false
	}

	if result.ExitCode != 0 {
		logMsg("agent", fmt.Sprintf("%s failed (%dms): %s", a.name, durationMs, truncateStr(result.Stderr, 200)))
		emitEvent(Event{
			Type:        "agent",
			Agent:       a.name,
			Status:      "failed",
			Prompt:      task,
			Error:       result.Stderr,
			DurationMs:  durationMs,
			Timestamp:   time.Now().Format(time.RFC3339),
		})
		return "", fmt.Errorf("agent run failed (exit %d): %s", result.ExitCode, result.Stderr)
	}

	output := strings.TrimSpace(result.Stdout)
	logMsg("agent", fmt.Sprintf("%s done (%.1fs)", a.name, float64(durationMs)/1000))

	emitEvent(Event{
		Type:       "agent",
		Agent:      a.name,
		Status:     "completed",
		Prompt:     task,
		Result:     output,
		DurationMs: durationMs,
		Timestamp:  time.Now().Format(time.RFC3339),
	})

	return output, nil
}

func deriveAgentName(systemPrompt string) string {
	words := strings.ToLower(systemPrompt)
	words = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			return r
		}
		return -1
	}, words)
	parts := strings.Fields(words)
	if len(parts) > 4 {
		parts = parts[:4]
	}
	joined := strings.Join(parts, "-")
	joined = strings.Trim(joined, "-")

	h := md5.Sum([]byte(systemPrompt))
	hash := hex.EncodeToString(h[:])[:8]

	if joined == "" {
		return hash
	}
	return joined + "-" + hash
}

func generateSessionID() string {
	now := time.Now()
	ts := fmt.Sprintf("%04d%02d%02d_%02d%02d%02d",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second())
	var b [4]byte
	rand.Read(b[:])
	suffix := hex.EncodeToString(b[:])
	return "openflow_session_" + ts + "_" + suffix
}
