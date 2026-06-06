package sdk

import (
	"fmt"
	"os"
)

type AgentOpts struct {
	Name         string
	SystemPrompt string
	AgentRunner  string
	Model        string
}

type Agent struct {
	err error
}

func NewAgent(opts AgentOpts) *Agent {
	if opts.Name == "" {
		msg := fmt.Sprintf("[openflow lint] agent missing name (systemPrompt=%q)\n", opts.SystemPrompt)
		fmt.Fprint(os.Stderr, msg)
		return &Agent{err: fmt.Errorf("%s", msg)}
	}
	return &Agent{}
}

type RunOpts struct {
	Feedbacks []Feedback
}

type AgentOutput struct {
	ShellSuggestions map[string]string
}

func (a *Agent) Run(task string, opts RunOpts) (string, *AgentOutput, error) {
	if a.err != nil {
		return "", nil, a.err
	}
	return "", &AgentOutput{}, nil
}
