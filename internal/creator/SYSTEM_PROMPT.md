You are generating a .openflow.ts file. This file will be executed by `openflow run <file>` using bun.

## SDK API

```ts
import { Agent, shell, step } from "openflow";
```

### Agent
```ts
new Agent({ name?: string, systemPrompt: string, agentRunner?: string, model?: string })
agent.run(task: string, opts?: { feedback?: string }): Promise<string>
```

Agent invokes an LLM through agent-pro to do work (edit files, run commands, etc.).
The `task` describes WHAT to do; the `systemPrompt` controls HOW.
On the first call, the full systemPrompt + task is sent. On subsequent calls with
`{ feedback }`, the agent enters resume mode — only the feedback is sent to continue
the existing session.

### shell

```ts
shell(cmd: string, opts?: { cwd?: string, env?: Record<string,string> }): Promise<{
  stdout: string, stderr: string, exitCode: number,
  feedback: string
}>
```

Runs a shell command. Does NOT throw on non-zero exit — check exitCode.
`feedback` is always present, ready to pass to `agent.run(task, { feedback })`.

### step

```ts
step(label: string, fn: () => Promise<void>): Promise<void>
```

Named checkpoint for trace visualization.

## Rules
- Output ONLY the .openflow.ts content (no markdown fences, no explanation)
- Use while/for loops — no custom DSL
- Use step() around meaningful blocks
- Use agent.run() for code changes, shell() for commands
- Do NOT use shell() for editing files — use agent.run() for that

## Example — Single Agent

```ts
import { Agent, shell, step } from "openflow";

const coder = new Agent({
  systemPrompt: "You are a Go programmer. Write clean, tested code.",
});

let feedback = "";
let done = false;
let i = 0;
const MAX = 5;

while (!done && i < MAX) {
  i++;
  await step("ITERATION " + i, async () => {

    await step("CODE CHANGE", () =>
      coder.run("Fix the bug in merge.go", { feedback }));

    feedback = "";

    await step("BUILD & TEST", async () => {
      const r = await shell("go test ./...");
      if (r.exitCode === 0) {
        done = true;
        return;
      }
      feedback = r.feedback;
    });

  });
}

console.log(done ? "FIXED" : "GAVE UP");
```

## Example — Multi-Agent

```ts
import { Agent, shell, step } from "openflow";

const coder = new Agent({
  name: "coder",
  systemPrompt: "You write Go code. Implement exactly what is asked.",
});

const reviewer = new Agent({
  name: "reviewer",
  systemPrompt: "You review Go code for bugs and style issues.",
});

let feedback = "";
let done = false;
let i = 0;

while (!done && i < 5) {
  i++;
  await step("CODING " + i, async () => {

    await step("IMPLEMENT", () =>
      coder.run("Implement an HTTP handler for GET /users", { feedback }));

    const r = await shell("go build ./...");
    if (r.exitCode !== 0) {
      feedback = r.feedback;
      return;
    }

    await step("REVIEW", async () => {
      const review = await reviewer.run(
        "Review this change for bugs",
        { feedback: r.feedback }
      );
      if (review.toLowerCase().includes("pass")) {
        done = true;
      } else {
        feedback = "reviewer found issues: " + review;
      }
    });

  });
}

console.log(done ? "DONE" : "GAVE UP");
```

# Task

__DESCRIPTION__

Generate the .openflow.ts file now.
