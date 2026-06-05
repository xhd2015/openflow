import { shell } from "./shell";
import { emitEvent, log, deriveAgentName, generateSessionID } from "./events";

export interface AgentOptions {
  name?: string;
  systemPrompt: string;
  agentRunner?: string;
  model?: string;
}

export interface RunOptions {
  feedback?: string;
}

export class Agent {
  readonly name: string;
  private systemPrompt: string;
  private agentRunner: string;
  private model: string;
  private sessionID: string;
  private isFirstRun: boolean;
  private lastTask: string;

  constructor(opts: AgentOptions) {
    this.systemPrompt = opts.systemPrompt;
    this.agentRunner = opts.agentRunner || process.env.OPENFLOW_AGENT_RUNNER || "opencode";
    this.model = opts.model || process.env.OPENFLOW_MODEL || "";
    this.name = opts.name || deriveAgentName(this.systemPrompt);
    this.sessionID = generateSessionID();
    this.isFirstRun = true;
    this.lastTask = "";
  }

  async run(task: string, opts?: RunOptions): Promise<string> {
    const feedback = opts?.feedback || "";
    const isResume = !this.isFirstRun && feedback !== "" && task === this.lastTask;
    this.lastTask = task;

    let prompt: string;
    if (isResume) {
      prompt = `# Feedback\n${feedback}`;
      log("agent", `${this.name} resume: "${truncateStr(feedback, 100)}"`);
    } else {
      prompt = `${this.systemPrompt}\n\n# Task\n${task}`;
      log("agent", `${this.name} run: "${truncateStr(task, 100)}"`);
    }

    const startMs = Date.now();

    emitEvent({
      type: "agent",
      agent: this.name,
      status: "started",
      prompt: task,
      resume: isResume,
      timestamp: new Date().toISOString(),
    });

    const args = [
      "exec",
      "--prompt", prompt,
      "--agent-runner", this.agentRunner,
      "--session", this.sessionID,
    ];
    if (this.model) {
      args.push("--model", this.model);
    }
    if (isResume) {
      args.push("--resume");
    }
    const dir = process.env.OPENFLOW_WORKSPACE || process.cwd();
    args.push("--dir", dir);

    const runID = process.env.OPENFLOW_RUN_ID;
    if (runID) {
      const runDir = `${process.env.OPENFLOW_HOME || "~/.openflow"}/runs/${runID}`;
      args.push("--trace-dir", `${runDir}/agents/${this.name}`);
    }

    const openflowBin = process.env.OPENFLOW_BIN || "openflow";
    const cmd = [openflowBin, ...args].map(quoteArg).join(" ");
    const result = await shell(cmd);

    const durationMs = Date.now() - startMs;

    if (this.isFirstRun) {
      this.isFirstRun = false;
    }

    if (result.exitCode !== 0) {
      log("agent", `${this.name} failed (${durationMs}ms): ${truncateStr(result.stderr, 200)}`);
      emitEvent({
        type: "agent",
        agent: this.name,
        status: "failed",
        prompt: task,
        error: result.stderr,
        duration_ms: durationMs,
        timestamp: new Date().toISOString(),
      });
      throw new Error(`agent run failed (exit ${result.exitCode}): ${result.stderr}`);
    }

    const output = result.stdout.trim();
    log("agent", `${this.name} done (${(durationMs / 1000).toFixed(1)}s)`);

    emitEvent({
      type: "agent",
      agent: this.name,
      status: "completed",
      prompt: task,
      result: output,
      duration_ms: durationMs,
      timestamp: new Date().toISOString(),
    });

    return output;
  }
}

function truncateStr(s: string, max: number): string {
  s = s.replace(/\s+/g, " ").trim();
  if (s.length <= max) return s;
  return s.slice(0, max - 3) + "...";
}

function quoteArg(arg: string): string {
  if (arg.includes(" ") || arg.includes('"') || arg.includes("'")) {
    return `'${arg.replace(/'/g, "'\\''")}'`;
  }
  return arg;
}
