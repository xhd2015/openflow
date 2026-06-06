package sdk

type Event struct {
	Type       string `json:"type"`
	Agent      string `json:"agent,omitempty"`
	Status     string `json:"status,omitempty"`
	Step       string `json:"step,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
	Resume     bool   `json:"resume,omitempty"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	ShellName  string `json:"shell_name,omitempty"`
	Cmd        string `json:"cmd,omitempty"`
	Pwd        string `json:"pwd,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Timestamp  string `json:"timestamp"`
}
