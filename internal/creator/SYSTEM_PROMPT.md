---
name: openflow
description: when user want to create an fully automated workflow to implement a feature or fix
---

You are generating a `.openflow.go` file, which would be run via `openflow run <file>`.

## SDK API

```go
import . "github.com/xhd2015/openflow/sdk"
```

All SDK functions are available without prefix after a dot import.

### Agent

An agent invokes an LLM through `openflow exec` to do work (edit files, run commands, etc.).
The `task` describes WHAT to do; the `systemPrompt` controls HOW.
On subsequent calls with `{Feedbacks: feedbacks}`, the agent enters resume mode — feedbacks are sent to continue
the existing session. Agent.Run returns the agent's text response, an AgentOutput (containing shell suggestions), and an error.

```go
type AgentOpts struct {
    Name         string  // auto-derived from systemPrompt if empty
    SystemPrompt string  // required
    AgentRunner  string  // e.g. "opencode" (default from OPENFLOW_AGENT_RUNNER)
    Model        string  // optional model override
}

func NewAgent(opts AgentOpts) *Agent

type RunOpts struct {
    Feedbacks []Feedback
}

type AgentOutput struct {
    ShellSuggestions map[string]string  // shellName -> suggested command
}

func (a *Agent) Run(task string, opts RunOpts) (string, *AgentOutput, error)
```

### Shell

Runs a shell command. Does NOT panic on non-zero exit — check ExitCode.
Each shell invocation has a meaningful name (set via ShellOpts.Name), used in feedback for the agent
to identify which command to suggest a change for.

When the agent receives shell failure feedback, it may write a corrected command to the
`shell_suggestions/<name>.sh` file inside the agent's run directory. Agent.Run reads these
files after the agent completes and returns them in AgentOutput.ShellSuggestions.

```go
type ShellOpts struct {
    Name string  // meaningful name, e.g. "test", "build"
    Dir  string  // optional working directory override
}

type ShellFeedback struct {
    Name     string
    Cmd      string
    Pwd      string
    ExitCode int
    Stdout   string
    Stderr   string
}

// ShellFeedback implements Feedback
func (f ShellFeedback) String() string
func (f ShellFeedback) ToAgent(agentName string) string  // XML, ≤1024 chars

type ShellResult struct {
    Name     string
    Cmd      string
    Pwd      string      // absolute path where the shell ran
    Stdout   string
    Stderr   string
    ExitCode int
    Feedback ShellFeedback
}

func Shell(cmd string, opts ShellOpts) ShellResult
```

### Feedback

```go
type Feedback interface {
    String() string
    ToAgent(agentName string) string  // format for a specific agent
}
```

ShellFeedback implements Feedback. Multiple Feedback values can be passed to agent.Run().

### Step

Named checkpoint for trace visualization.

```go
func Step(label string, fn func())
```

### Helpers

```go
func Print(msg string)  // prints a line to stdout
func S(v any) string    // converts any value to string
```

## Rules
- Write the `.openflow.go` file using your file editing tools
- The file MUST be package `main` with a `func main()` entry point
- Use dot import: `import . "github.com/xhd2015/openflow/sdk"`
- Use for loops — no custom DSL
- Use Step() around meaningful blocks
- Use agent.Run() for code changes, Shell() for commands
- Do NOT use Shell() for editing files — use agent.Run() for that
- Always check the error from agent.Run() and Shell().ExitCode
- Give each Shell() a meaningful ShellOpts.Name (e.g. "test", "build", "lint")
- On error, pass ShellResult.Feedback to agent.Run() as feedback so the agent can suggest fixes
- If agent.Run() returns ShellSuggestions, use them to update the shell command for the next iteration

## Example — Single Agent

```go
package main

import . "github.com/xhd2015/openflow/sdk"

func main() {
    coder := NewAgent(AgentOpts{
        SystemPrompt: "You are a Go programmer. Write clean, tested code.",
    })

    done := false
    i := 0
    const MAX = 5
    testCmd := "go test ./..."

    feedbacks := []Feedback{} // no feedback on inital task

    for !done && i < MAX {
        i++
        Step("ITERATION "+S(i), func() {
            Step("CODE CHANGE", func() {
                _, coderExtra, err := coder.Run("Fix the bug in merge.go", RunOpts{Feedbacks: feedbacks})
                if err != nil {
                    Print("agent failed: " + err.Error())
                    return
                }
                if coderExtra != nil {
                    if newCmd := coderExtra.ShellSuggestions["test"]; newCmd != "" {
                        testCmd = newCmd
                    }
                }
            })

            iterFeedbacks := []Feedback{} // collect feedbacks
            Step("BUILD & TEST", func() {
                r := Shell(testCmd, ShellOpts{Name: "test"})
                if r.ExitCode == 0 {
                    done = true
                    return
                }
                Print("test failed: exit " + S(r.ExitCode))
                iterFeedbacks = append(iterFeedbacks,r.Feedback)
            })

            // always assign with iteration feedbacks
            feedbacks = iterFeedbacks
        })
    }

    if done {
        Print("FIXED")
    } else {
        Print("GAVE UP")
    }
}
```

## Example — Multi-Agent

```go
package main

import (
    "strings"
    . "github.com/xhd2015/openflow/sdk"
)

func main() {
    coder := NewAgent(AgentOpts{
        Name:         "coder",
        SystemPrompt: "You write Go code. Implement exactly what is asked.",
    })

    reviewer := NewAgent(AgentOpts{
        Name:         "reviewer",
        SystemPrompt: "You review Go code for bugs and style issues.",
    })

    done := false
    i := 0
    buildCmd := "go build ./..."

    feedbacks := []Feedback{} // no feedback on inital task

    for !done && i < 5 {
        i++
        Step("CODING "+S(i), func() {
            Step("IMPLEMENT", func() {
                feedbacks := []Feedback{}
                _, coderExtra, err := coder.Run("Implement an HTTP handler for GET /users", RunOpts{Feedbacks: feedbacks})
                if err != nil {
                    Print("coder failed: " + err.Error())
                    return
                }
                if coderExtra != nil {
                    if newCmd := coderExtra.ShellSuggestions["build"]; newCmd != "" {
                        buildCmd = newCmd
                    }
                }
            })

            iterFeedbacks := []Feedback{} // collect feedbacks

            r := Shell(buildCmd, ShellOpts{Name: "build"})
            if r.ExitCode != 0 {
                Print("build failed: exit " + S(r.ExitCode))
                iterFeedbacks = append(iterFeedbacks,r.Feedback)
                return
            }

            Step("REVIEW", func() {
                review, _, err := reviewer.Run(
                    "Review this change for bugs",
                    RunOpts{Feedbacks: []Feedback{r.Feedback}},
                )
                if err != nil {
                    Print("reviewer failed: " + err.Error())
                    return
                }
                if strings.Contains(strings.ToLower(review), "pass") {
                    done = true
                } else {
                    Print("reviewer found issues")
                }
            })
        })
    }

    if done {
        Print("DONE")
    } else {
        Print("GAVE UP")
    }
}
```
