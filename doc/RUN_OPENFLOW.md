# Running Openflows

```
openflow run <openflow-file> [--skip-validate] [--trace-dir DIR] [--timeout SECONDS]
```

## Execution Pipeline

```
openflow run file.openflow.ts
  │
  ├─ 1. Resolve absolute path of openflow file
  │
  ├─ 2. Validate (unless --skip-validate)
  │     Runs: tsc --noEmit --strict file.openflow.ts
  │     (bun can also type-check: bun run --check file.openflow.ts)
  │     If validation fails → print errors, exit 1
  │
  ├─ 3. Extract embedded SDK
  │     SDK is embedded in the openflow Go binary via //go:embed
  │     Extracted to ~/.openflow/sdk/ on first run (cached)
  │     If outdated, re-extract
  │
  ├─ 4. Create run record
  │     Directory: ~/.openflow/runs/<timestamp>-<run-id>/
  │     Writes metadata.json with status=running
  │
  ├─ 5. Set up environment
  │     OPENFLOW_HOME=~/.openflow
  │     OPENFLOW_RUN_ID=<run-id>
  │     OPENFLOW_TRACE_ID=<trace-id>
  │     OPENFLOW_TRACE_DIR=~/.openflow/agent-traces/<trace-id>
  │     OPENFLOW_SDK_DIR=~/.openflow/sdk
  │     (also passes through: PATH, HOME, etc.)
  │
  ├─ 6. Generate import wrapper
  │     Creates a temp wrapper .ts file that:
  │       - Sets up import paths for the SDK
  │       - Then dynamically imports the user's openflow file
  │     This avoids requiring users to know the SDK location
  │
  └─ 7. Execute
       Runs: bun run <wrapper> --openflow-file=<file>
       Inherits stdin/stdout/stderr
       Captures exit code
       On completion: updates metadata.json status to completed|failed
```

### Import Resolution — How the user file gets the SDK

The user's `.openflow.ts` writes:

```typescript
import { Agent, shell, step } from "openflow";
```

But `openflow` is not an npm package. The Go binary resolves this:

**Approach**: The SDK lives in `~/.openflow/sdk/` (extracted from the binary). A generated wrapper script preloads it.

```typescript
// Generated wrapper: /tmp/openflow-runner-<hash>.ts
// Sets up Node.js module resolution so "openflow" resolves to the SDK

import { register } from "node:module";
import { pathToFileURL } from "node:url";

// Register the SDK directory as a package
// Then load the user's file
await import(process.env.OPENFLOW_USER_FILE!);
```

**Alternative (simpler)**: The `openflow` binary writes a tiny `package.json` in the user's project:

```json
{
  "dependencies": {
    "openflow": "file:~/.openflow/sdk"
  }
}
```

The wrapper calls `bun install` if needed, then `bun run <file>`.

**Decision**: Start with the `package.json` approach — simplest, most debuggable. The wrapper:
1. Checks if `node_modules/openflow` exists in the openflow file's directory
2. If not, writes/updates `<project>/node_modules/openflow` as a symlink to `~/.openflow/sdk`
3. Or: generates a temp `package.json` and runs `bun install --no-save` before the openflow

---

## SDK Primitives (Runtime Behavior)

### Agent

```typescript
const agent = new Agent({
  systemPrompt: "You are a Go programmer.",
  agentRunner: "opencode",   // default from env OPENFLOW_AGENT_RUNNER or "opencode"
  model: "deepseek/deepseek-v4-pro",
});

const result = await agent.run("Write a function that...");
```

**Under the hood**: `agent.run()` spawns an agent-pro runner subprocess:

```
Agent.run(prompt) →
  1. Writes system_prompt + prompt to temp file
  2. Spawns: codenn [--agent-runner <runner>] [--model <model>] --parent-trace-id <OPENFLOW_TRACE_ID> <prompt>
     (or uses agent-pro registry.Agent.Ask directly — TBD based on what's simpler)
  3. Captures stdout → returns as result string
  4. Writes agent-calls.jsonl entry to the run directory
```

**`agent.runAsync()`** spawns the agent in the background, returns immediately.
The openflow can later `await` the promise.

### shell

```typescript
const { stdout, stderr, exitCode } = await shell("git push");
```

**Under the hood**: `shell()` uses `Bun.spawnSync()` (or `child_process.execSync`):

```
shell(cmd, opts?) →
  1. Spawns /bin/sh -c <cmd>
  2. Waits for exit (timeout: 5 min default, configurable via opts.timeout)
  3. Returns { stdout, stderr, exitCode }
  4. Does NOT throw on non-zero exit — caller inspects exitCode
```

### step

```typescript
await step("CODE CHANGE", async () => {
  // work happens here
});
```

**Under the hood**: `step()` emits trace events:

```
step(label, fn) →
  1. Emit { step: label, status: "started", timestamp } to steps.jsonl
  2. Execute fn()
  3. Emit { step: label, status: "completed|failed", duration_ms, error? } to steps.jsonl
  4. If fn() throws: emit failed, re-throw
```

---

## Status & Inspection

### `openflow status [run-id]`

Prints terminal summary:

```
$ openflow status

RUN ID              STATUS      FILE                          STEPS  DURATION
20260605-abc123     completed   ci-debug.openflow.ts          5      12m34s
20260605-def456     failed      broken.openflow.ts            2      0m03s

$ openflow status abc123

status: completed
file: ci-debug.openflow.ts
steps:
  1. CODE CHANGE        completed  2m15s
  2. UPDATE YAML        completed  0m45s
  3. COMMIT & PUSH      completed  0m12s
  4. FETCH CI           completed  3m02s
  5. DECIDE             completed  0m01s
exit: 0
```

### `openflow trace [run-id]`

Opens the agent-pro web trace viewer (like `murphy agent-traces`), focused on this run's trace.
Shows the openflow steps as timeline entries, with child agent calls nested under each step.

### `openflow logs [run-id]`

Prints the raw stdout/stderr from the openflow execution.

---

## Environment Variables

Variables set by `openflow run` before invoking `bun`:

| Variable | Value | Used by |
|----------|-------|---------|
| `OPENFLOW_HOME` | `~/.openflow` | SDK persistence |
| `OPENFLOW_RUN_ID` | `<run-id>` | SDK trace events |
| `OPENFLOW_TRACE_ID` | `<trace-id>` | agent-pro trace linking |
| `OPENFLOW_TRACE_DIR` | `<trace-dir>` | agent-pro raw log destination |
| `OPENFLOW_SDK_DIR` | `~/.openflow/sdk` | module resolution |
| `OPENFLOW_TIMEOUT` | configurable | global timeout for openflow |
| `OPENFLOW_AGENT_RUNNER` | default runner | Agent default |
| `OPENFLOW_MODEL` | default model | Agent default |

---

## Timeout & Cancellation

- `--timeout SECONDS` (default: no limit) — kills the bun process after timeout
- `agent.run()` has a per-call timeout (default: 10 min, configurable via `AgentOptions.timeout`)
- `shell()` has a per-call timeout (default: 5 min, configurable via `opts.timeout`)
- SIGINT / Ctrl+C: forwards to bun, which forwards to the openflow; `step()`/`agent.run()`/`shell()` throw

---

## Error Handling in Openflows

```typescript
try {
  const result = await agent.run("fix bug");
} catch (e) {
  console.error("Agent failed:", e.message);
  // Decide: retry, give up, try different approach
}
```

```typescript
const { exitCode, stderr } = await shell("go test ./...");
if (exitCode !== 0) {
  await step("FIX TESTS", () =>
    agent.run("Test failure: " + stderr));
}
```

The openflow SDK does NOT auto-retry. The openflow author controls retry logic in TypeScript.

---

## Example: Full Execution Trace

For this openflow:
```typescript
import { Agent, shell, step } from "openflow";
const agent = new Agent({ systemPrompt: "You fix bugs." });

await step("FIX", () => agent.run("Fix the nil pointer"));
const { stdout } = await shell("go test ./...");
console.log(stdout);
```

The trace would look like:

```
~/.openflow/runs/20260605-abc123/
  metadata.json          → status=completed, exit_code=0
  steps.jsonl            → "FIX": started → completed
  agent-calls.jsonl      → agent=default, prompt="Fix the nil pointer", status=completed
  stdout.txt             → (go test output)
  stderr.txt             → (empty)
```

And in agent-traces:
```
~/.openflow/agent-traces/<trace-id>/
  metadata.json          → parent openflow metadata
  events.jsonl           → raw agent-pro events from the codenn subprocess
```
