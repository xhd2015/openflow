package sdk

import (
	"fmt"
	"os"
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
	return "[shell " + f.Name + "] (mock)"
}

func (f ShellFeedback) ToAgent(agentName string) string {
	return "<bash name=\"" + f.Name + "\">\nmock\n</bash>"
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
	if opts.Name == "" {
		msg := fmt.Sprintf("[openflow lint] shell missing name: %q\n", cmdStr)
		fmt.Fprint(os.Stderr, msg)
		return ShellResult{
			Cmd:      cmdStr,
			ExitCode: 1,
			Stderr:   msg,
		}
	}
	return ShellResult{
		Name:     opts.Name,
		Cmd:      cmdStr,
		ExitCode: 0,
		Feedback: ShellFeedback{
			Name: opts.Name,
			Cmd:  cmdStr,
		},
	}
}
