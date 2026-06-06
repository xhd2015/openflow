package sdk

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Feedbacks []Feedback
}

type AgentOutput struct {
	ShellSuggestions map[string]string
}

func (a *Agent) Run(task string, opts RunOpts) (string, *AgentOutput, error) {
	isResume := !a.isFirstRun && len(opts.Feedbacks) > 0 && task == a.lastTask
	a.lastTask = task

	home := envOr("OPENFLOW_HOME", "")
	runID := os.Getenv("OPENFLOW_RUN_ID")

	var msgDir string
	if home != "" && runID != "" {
		msgDir = filepath.Join(home, "runs", runID, "agents", a.name)
		os.MkdirAll(msgDir, 0755)
	}

	var prompt string
	if isResume {
		var parts []string
		for _, fb := range opts.Feedbacks {
			fbStr := fb.ToAgent(a.name)
			parts = append(parts, fbStr)
			logMsg("agent", a.name+" feedback: \""+truncateStr(fbStr, 1024)+"\"")
			writeAgentMessage(msgDir, "feedback", fbStr)
		}
		prompt = "# Feedback\n" + strings.Join(parts, "\n\n")
	} else {
		prompt = a.systemPrompt + "\n\n# Task\n" + task
		if a.isFirstRun {
			logMsg("agent", a.name+" run: \""+truncateStr(task, 1024)+"\"")
		} else {
			logMsg("agent", a.name+" run (no feedback, fresh start): \""+truncateStr(task, 1024)+"\"")
		}
		writeAgentMessage(msgDir, "prompt", task)
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

	suggestionDir := ""
	if home != "" && runID != "" {
		suggestionDir = filepath.Join(home, "runs", runID, "agents", a.name, "shell_suggestions")
		os.MkdirAll(suggestionDir, 0755)
	}

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

	if runID != "" {
		args = append(args, "--trace-dir", filepath.Join(home, "runs", runID, "agents", a.name))
	}

	openflowBin := envOr("OPENFLOW_BIN", "openflow")
	cmdStr := openflowBin
	for _, arg := range args {
		cmdStr += " " + quoteArg(arg)
	}

	result := Shell(cmdStr, ShellOpts{Name: a.name + "-exec"})
	durationMs := time.Since(startMs).Milliseconds()

	if a.isFirstRun {
		a.isFirstRun = false
	}

	if result.ExitCode != 0 {
		logMsg("agent", fmt.Sprintf("%s failed (%dms): %s", a.name, durationMs, truncateStr(result.Stderr, 200)))
		emitEvent(Event{
			Type:       "agent",
			Agent:      a.name,
			Status:     "failed",
			Prompt:     task,
			Error:      result.Stderr,
			DurationMs: durationMs,
			Timestamp:  time.Now().Format(time.RFC3339),
		})
		return "", nil, fmt.Errorf("agent run failed (exit %d): %s", result.ExitCode, result.Stderr)
	}

	output := strings.TrimSpace(result.Stdout)
	logMsg("agent", fmt.Sprintf("%s done (%.1fs)", a.name, float64(durationMs)/1000))

	writeAgentMessage(msgDir, "output", truncateStr(output, 1024))

	printOutput := truncateStr(output, 1024)
	if printOutput != "" {
		Print(printOutput)
	}

	emitEvent(Event{
		Type:       "agent",
		Agent:      a.name,
		Status:     "completed",
		Prompt:     task,
		Result:     output,
		DurationMs: durationMs,
		Timestamp:  time.Now().Format(time.RFC3339),
	})

	agentOutput := &AgentOutput{
		ShellSuggestions: make(map[string]string),
	}

	if suggestionDir != "" {
		entries, err := os.ReadDir(suggestionDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if !strings.HasSuffix(name, ".sh") || strings.Contains(name, "_") {
					continue
				}
				shellName := strings.TrimSuffix(name, ".sh")
				content, err := os.ReadFile(filepath.Join(suggestionDir, name))
				if err != nil {
					continue
				}
				suggestedCmd := strings.TrimSpace(string(content))
				if suggestedCmd != "" {
					agentOutput.ShellSuggestions[shellName] = suggestedCmd
					logMsg("agent", fmt.Sprintf("%s suggestion: %s => %s", a.name, shellName, truncateStr(suggestedCmd, 1024)))
				}
				ts := time.Now().Format("20060102_150405")
				archivedName := shellName + "_" + ts + ".sh"
				os.Rename(filepath.Join(suggestionDir, name), filepath.Join(suggestionDir, archivedName))
			}
		}
	}

	return output, agentOutput, nil
}

func writeAgentMessage(msgDir, msgType, content string) {
	if msgDir == "" {
		return
	}
	msg := struct {
		Type      string `json:"type"`
		Content   string `json:"content"`
		Timestamp string `json:"timestamp"`
	}{
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(msgDir, "messages.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
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
