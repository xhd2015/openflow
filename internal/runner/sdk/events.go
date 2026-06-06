package sdk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var (
	runDir      string
	runDirOnce  sync.Once
	runDirReady bool
)

func ensureRunDir() {
	runDirOnce.Do(func() {
		runID := os.Getenv("OPENFLOW_RUN_ID")
		if runID == "" {
			return
		}
		home := os.Getenv("OPENFLOW_HOME")
		if home == "" {
			home, _ = os.UserHomeDir()
			home = filepath.Join(home, ".openflow")
		}
		runDir = filepath.Join(home, "runs", runID)
		os.MkdirAll(runDir, 0755)
		runDirReady = true
	})
}

type Event struct {
	Type       string `json:"type"`
	Agent      string `json:"agent,omitempty"`
	Status     string `json:"status,omitempty"`
	Step       string `json:"step,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
	Resume     bool   `json:"resume,omitempty"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	Cmd        string `json:"cmd,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Timestamp  string `json:"timestamp"`
}

func emitEvent(e Event) {
	ensureRunDir()
	if runDir == "" || !runDirReady {
		return
	}
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(runDir, "events.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}
