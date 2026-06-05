# Creating Openflows

A `.openflow.ts` file can be created in two ways: CLI-driven (`openflow create`) or agent-driven
(any codenn/codex agent writes the file). Both paths produce the same output: a valid `.openflow.ts`
that can be executed with `openflow run`.

---

## Path A: CLI-Driven (`openflow create`)

```
openflow create <description> [--out FILE] [--run] [--agent-runner RUNNER] [--model MODEL]
```

### How it works

1. The CLI reads the built-in openflow SDK API reference (same content as `OPENFLOW.md` below)
2. Constructs a prompt: "Create a `.openflow.ts` file that does: `<description>`"
3. Calls the configured agent-runner (default: opencode) via agent-pro
4. Writes the generated `.openflow.ts` to `--out` (default: `<slug>.openflow.ts`)
5. If `--run` is specified, immediately executes `openflow run <out-file>`

### Example

```bash
openflow create "debug CI in xgo, keep trying until green" --out ci-debug.openflow.ts --run
```

---

## Path B: Agent-Driven Creation

Any agent (codenn, murphy, codex) that can read files and write files can create a `.openflow.ts`.
The agent needs to understand the openflow SDK API — this knowledge comes from `OPENFLOW.md`.

### Setup: place OPENFLOW.md in the repo

Place this file in the repo root (or any parent directory — agents resolve it upward like `CODENN.md`).

### OPENFLOW.md content (the instruction file)

```markdown
# Openflow Design Instructions

You are a openflow designer. When asked to create a openflow, write a `.openflow.ts` file
using the openflow SDK primitives below. The file will be executed by `openflow run <file>`.

## SDK API

All primitives are imported from `openflow`:

```typescript
import { Agent, shell, step } from "openflow";
```

### Agent

Creates an LLM-backed agent that runs coding tasks.

```typescript
const agent = new Agent({
  systemPrompt: string,      // required: system instructions for this agent
  agentRunner?: string,      // "codex" | "opencode" | "cursor" (default: "opencode")
  model?: string,            // model override (e.g. "deepseek/deepseek-v4-pro")
});

// Run a task and wait for result
const result: string = await agent.run(prompt: string);

// Run a task asynchronously (use for parallel work)
const result: string = await agent.runAsync(prompt: string);
```

The agent writes code, edits files, reads files, runs commands — just like codenn.
The prompt should describe WHAT to do, not HOW (the agent's system_prompt handles approach).

### shell

Runs a shell command and captures output. Use for git, building, testing, fetching data.

```typescript
const result: ShellResult = await shell(cmd: string, opts?: {
  cwd?: string;
  env?: Record<string, string>;
});

// result.stdout, result.stderr, result.exitCode
```

Shell commands have a default timeout of 5 minutes. Non-zero exit codes do NOT throw —
inspect `result.exitCode` and decide.

### step

Named checkpoint that emits a trace event for visualization. Wraps a block of work.

```typescript
await step(label: string, fn: () => Promise<void>);
```

Step labels appear in `openflow status` and the trace viewer. Use descriptive names:
"CODE CHANGE", "BUILD & TEST", "COMMIT & PUSH", "FETCH CI LOGS".

## Patterns

### Loop Until Success

```typescript
let done = false;
let i = 0;
while (!done && i < MAX_ITERATIONS) {
  i++;
  await step("ITERATION " + i, async () => {
    await agent.run("fix the issue based on CI logs");
    await shell("git add -A && git commit -m 'fix' && git push");
    const ci = await shell("github-fetch pr --logs '...' --openflow '...'");
    done = !ci.stdout.includes("FAIL");
  });
}
console.log(done ? "FIXED" : "GAVE UP");
```

### Multi-Agent Pipeline

```typescript
const coder = new Agent({ systemPrompt: "You write Go code." });
const reviewer = new Agent({ systemPrompt: "You review Go code for bugs." });

for (let i = 0; i < 3; i++) {
  const code = await coder.run("Write function for: " + task);
  const review = await reviewer.run("Review: " + code);
  if (review.includes("PASS")) break;
  task = "Fix issues: " + review;
}
```

### Shell-Based Validation

```typescript
const { exitCode, stdout } = await shell("go test ./...");
if (exitCode !== 0) {
  await agent.run("Fix failing tests: " + stdout);
}
```

## Rules

1. Use `while`/`for` loops — no custom loop DSL
2. Always set a maximum iteration count to prevent infinite loops
3. Use `step()` around meaningful blocks for trace visibility
4. `shell()` for git, build, test, fetch — never for direct file edits
5. `agent.run()` for code changes, analysis, review
6. Handle errors: `shell()` does not throw on non-zero exit — check `exitCode`
7. Output a clear final message: "FIXED", "GAVE UP after N iterations", etc.
```

### How the agent uses OPENFLOW.md

1. Agent reads `OPENFLOW.md` (auto-resolved from cwd upward, same mechanism as `CODENN.md`/`MURPHY.md`)
2. User message: "Create a openflow that debugs CI in xgo"
3. Agent generates `ci-debug.openflow.ts` using the SDK primitives documented in OPENFLOW.md
4. Agent can optionally shell to `openflow run ci-debug.openflow.ts` to test
5. Agent can read openflow output and edit the `.openflow.ts` iteratively

---

## Iterative Improvement Cycle

An agent creating a openflow can also improve it:

```
Agent writes .openflow.ts
  → openflow run <file>
  → reads run output / status
  → if openflow gave up or failed:
      edits .openflow.ts (fix agent prompt, add handling, adjust conditions)
      → openflow run <file> (again)
```

This means `OPENFLOW.md` should also document how to **debug a openflow**:

```markdown
## Debugging a Openflow

When the openflow fails or gives up:

1. Read `openflow status <run-id>` to see which steps succeeded/failed
2. Read `openflow trace <run-id>` to see agent calls and their outputs
3. Common fixes:
   - Agent gave irrelevant output → tighten agent.systemPrompt
   - Shell command hung → add timeout to shell() call
   - Infinite loop → check exit condition logic
   - CI flakes → add retry logic around shell() calls
4. Re-run: `openflow run <file>` after edits
```

---

## Comparison: Path A vs Path B

| | Path A: `openflow create` | Path B: Agent writes file |
|---|---|---|
| Trigger | User CLI command | User message to agent |
| Who generates | openflow CLI → agent-pro | codenn/codex/murphy agent |
| Setup needed | None | OPENFLOW.md in repo |
| Usable for | One-shot openflow generation | Iterative create-edit-rerun cycle |
| Combined | `openflow create --run` runs immediately | Agent runs `openflow run` itself |
