# openflow

Orchestrate LLM agents and shell commands in TypeScript loops.

Write a `.openflow.ts` file using `Agent`, `shell`, and `step`. Run it. It keeps trying until done.

## Install

```bash
go install github.com/xhd2015/openflow@latest
```

Requires: `bun`
Agent Runner: `opencode` (or `codex`/`cursor`).

## Usage

### Create a workflow

```bash
openflow create "Fix the bug in merge.go and verify tests pass" --out fix.openflow.ts
```

Generates a `.openflow.ts` from a description using an LLM.

### Run a workflow

```bash
openflow run fix.openflow.ts
```

Executes the `.openflow.ts` with `bun`. Output streams in real-time.

## Quick example

```ts
import { Agent, shell, step } from "openflow";

const coder = new Agent({
  systemPrompt: "You fix Go bugs.",
});

let feedback = "";
let done = false;

while (!done && i < 5) {
  i++;
  await step("FIX " + i, () =>
    coder.run("Fix MergeJSON recursive merge", { feedback }));

  const r = await shell("go test ./...");
  if (r.exitCode === 0) done = true;
  else feedback = r.feedback;
}
```

## SDK

| Primitive | What it does |
|-----------|-------------|
| `new Agent({ name?, systemPrompt, agentRunner?, model? })` | Create an LLM agent |
| `agent.run(task, { feedback? })` | Send task to agent, return result. On 2nd+ call with `feedback`, enters resume mode. |
| `shell(cmd, { cwd?, env?, timeout? })` | Run a shell command. Returns `{ stdout, stderr, exitCode, feedback }`. |
| `step(label, fn)` | Named checkpoint with trace event emission. |

## View traces

```bash
agent-pro traces ~/.openflow/runs/<run-id>/agents/<name>/events.jsonl --print
```

See the full step-by-step timeline of what the agent did.

## Docs

- [Design](doc/DESIGN.md)
- [Creating workflows](doc/CREATE_OPENFLOW.md)
- [Running workflows](doc/RUN_OPENFLOW.md)
- [Storage layout](doc/RUN_STORAGE_LAYOUT.md)
