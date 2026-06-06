package sdk

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ShellOpts struct {
	Name string
	Dir  string
}

type ShellFeedback struct {
	Name     string
	Cmd      string
	Pwd      string
	ExitCode int
	Stdout   string
	Stderr   string
}

func (f ShellFeedback) String() string {
	return fmt.Sprintf("[shell %s] %s (exit %d)", f.Name, f.Cmd, f.ExitCode)
}

func (f ShellFeedback) ToAgent(agentName string) string {
	home := envOr("OPENFLOW_HOME", "")
	runID := os.Getenv("OPENFLOW_RUN_ID")

	var suggestionLine string
	if home != "" && runID != "" && agentName != "" && f.Name != "" {
		suggestionPath := filepath.Join(home, "runs", runID, "agents", agentName, "shell_suggestions", f.Name+".sh")
		suggestionLine = fmt.Sprintf("suggestion: If the command is incorrect, write a corrected version to: %s\n", suggestionPath)
	}

	body := fmt.Sprintf("cmd: %s\npwd: %s\nexit_code: %d\nstdout: %s\nstderr: %s\n",
		f.Cmd, f.Pwd, f.ExitCode, f.Stdout, f.Stderr)

	full := fmt.Sprintf("<bash name=\"%s\">\n%s%s</bash>", f.Name, body, suggestionLine)

	maxLen := 1024
	if len(full) <= maxLen {
		return full
	}

	bodyBudget := maxLen - len(fmt.Sprintf("<bash name=\"%s\">\n\n</bash>", f.Name)) - len(suggestionLine)
	if bodyBudget < 100 {
		bodyBudget = 100
	}
	halfBudget := bodyBudget / 2
	stdoutTrunc := truncateStr(f.Stdout, halfBudget)
	stderrTrunc := truncateStr(f.Stderr, halfBudget)
	truncBody := fmt.Sprintf("cmd: %s\npwd: %s\nexit_code: %d\nstdout: %s\nstderr: %s\n",
		truncateStr(f.Cmd, 200), f.Pwd, f.ExitCode, stdoutTrunc, stderrTrunc)
	result := fmt.Sprintf("<bash name=\"%s\">\n%s%s</bash>", f.Name, truncBody, suggestionLine)
	if len(result) > maxLen {
		result = result[:maxLen-3] + "..."
	}
	return result
}

type ShellResult struct {
	Name     string
	Cmd      string
	Pwd      string
	Stdout   string
	Stderr   string
	ExitCode int
	Feedback ShellFeedback
}

func Shell(cmdStr string, opts ShellOpts) ShellResult {
	startMs := time.Now()

	shellDir := opts.Dir
	if shellDir == "" {
		shellDir, _ = os.Getwd()
	}
	absPwd, _ := filepath.Abs(shellDir)

	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Dir = absPwd
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	var exitCode int
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
			stderr.WriteString(err.Error())
		}
	}

	durationMs := time.Since(startMs).Milliseconds()
	ts := time.Now().Format(time.RFC3339)

	outStr := stdout.String()
	errStr := stderr.String()

	if exitCode != 0 {
		errMsg := strings.TrimSpace(errStr)
		if errMsg == "" {
			errMsg = fmt.Sprintf("exit %d", exitCode)
		}
		logMsg("shell", truncateStr(fmt.Sprintf("[%s] %s", opts.Name, cmdStr), 80)+" FAILED: "+errMsg)
	} else {
		logMsg("shell", fmt.Sprintf("[%s] %s -> exit 0 (%dms)", opts.Name, truncateStr(cmdStr, 60), durationMs))
	}

	emitEvent(Event{
		Type:       "shell",
		ShellName:  opts.Name,
		Cmd:        cmdStr,
		Pwd:        absPwd,
		ExitCode:   exitCode,
		Stdout:     outStr,
		Stderr:     errStr,
		DurationMs: durationMs,
		Timestamp:  ts,
	})

	feedback := ShellFeedback{
		Name:     opts.Name,
		Cmd:      cmdStr,
		Pwd:      absPwd,
		ExitCode: exitCode,
		Stdout:   outStr,
		Stderr:   errStr,
	}

	return ShellResult{
		Name:     opts.Name,
		Cmd:      cmdStr,
		Pwd:      absPwd,
		Stdout:   outStr,
		Stderr:   errStr,
		ExitCode: exitCode,
		Feedback: feedback,
	}
}
