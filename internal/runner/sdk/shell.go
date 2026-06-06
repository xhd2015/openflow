package sdk

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ShellResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Feedback string
}

func Shell(cmdStr string) ShellResult {
	startMs := time.Now()

	cmd := exec.Command("sh", "-c", cmdStr)
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
		logMsg("shell", truncateStr(cmdStr, 80)+" FAILED: "+errMsg)
	} else {
		logMsg("shell", fmt.Sprintf("%s → exit 0 (%dms)", truncateStr(cmdStr, 80), durationMs))
	}

	emitEvent(Event{
		Type:       "shell",
		Cmd:        cmdStr,
		ExitCode:   exitCode,
		Stdout:     outStr,
		Stderr:     errStr,
		DurationMs: durationMs,
		Timestamp:  ts,
	})

	feedback := fmt.Sprintf("%s\nexit code: %d\n%s%s", truncateStr(cmdStr, 200), exitCode, outStr, errStr)
	feedback = strings.TrimSpace(feedback)

	return ShellResult{
		Stdout:   outStr,
		Stderr:   errStr,
		ExitCode: exitCode,
		Feedback: feedback,
	}
}
