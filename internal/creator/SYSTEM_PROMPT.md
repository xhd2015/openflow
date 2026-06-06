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
On the first call, the full systemPrompt + task is sent. On subsequent calls with
`{Feedback: feedback}`, the agent enters resume mode — only the feedback is sent to continue
the existing session.

```go
type AgentOpts struct {
    Name         string  // auto-derived from systemPrompt if empty
    SystemPrompt string  // required
    AgentRunner  string  // e.g. "opencode" (default from OPENFLOW_AGENT_RUNNER)
    Model        string  // optional model override
}

func NewAgent(opts AgentOpts) *Agent

type RunOpts struct {
    Feedback string
}

func (a *Agent) Run(task string, opts RunOpts) (string, error)
```

### Shell

Runs a shell command. Does NOT panic on non-zero exit — check ExitCode.
`Feedback` is always present, ready to pass to `agent.Run(task, RunOpts{Feedback: feedback})`.

```go
type ShellResult struct {
    Stdout   string
    Stderr   string
    ExitCode int
    Feedback string
}

func Shell(cmd string) ShellResult
```

### Step

Named checkpoint for trace visualization.

```go
func Step(label string, fn func())
```

### Helpers

```go
func Print(msg string)  // prints a line to stdout
func S(v int) string    // int to string
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
- On error, Print() a message and use the error/feedback for the next iteration

## Example — Single Agent

```go
package main

import . "github.com/xhd2015/openflow/sdk"

func main() {
    coder := NewAgent(AgentOpts{
        SystemPrompt: "You are a Go programmer. Write clean, tested code.",
    })

    feedback := ""
    done := false
    i := 0
    const MAX = 5

    for !done && i < MAX {
        i++
        Step("ITERATION "+S(i), func() {
            Step("CODE CHANGE", func() {
                _, err := coder.Run("Fix the bug in merge.go", RunOpts{Feedback: feedback})
                if err != nil {
                    Print("agent failed: " + err.Error())
                    feedback = err.Error()
                    return
                }
            })
            feedback = ""

            Step("BUILD & TEST", func() {
                r := Shell("go test ./...")
                if r.ExitCode == 0 {
                    done = true
                    return
                }
                Print("test failed: exit " + S(r.ExitCode))
                feedback = r.Feedback
            })
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

    feedback := ""
    done := false
    i := 0

    for !done && i < 5 {
        i++
        Step("CODING "+S(i), func() {
            Step("IMPLEMENT", func() {
                _, err := coder.Run("Implement an HTTP handler for GET /users", RunOpts{Feedback: feedback})
                if err != nil {
                    Print("coder failed: " + err.Error())
                    feedback = err.Error()
                    return
                }
            })

            r := Shell("go build ./...")
            if r.ExitCode != 0 {
                Print("build failed: exit " + S(r.ExitCode))
                feedback = r.Feedback
                return
            }

            Step("REVIEW", func() {
                review, err := reviewer.Run(
                    "Review this change for bugs",
                    RunOpts{Feedback: r.Feedback},
                )
                if err != nil {
                    Print("reviewer failed: " + err.Error())
                    feedback = err.Error()
                    return
                }
                if strings.Contains(strings.ToLower(review), "pass") {
                    done = true
                } else {
                    Print("reviewer found issues")
                    feedback = "reviewer found issues: " + review
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
