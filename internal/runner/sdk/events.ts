import { appendFileSync, mkdirSync } from "node:fs";
import { createHash, randomBytes } from "node:crypto";

const runID = process.env.OPENFLOW_RUN_ID || "";
const runDir = runID
  ? `${process.env.OPENFLOW_HOME || "~/.openflow"}/runs/${runID}`
  : "";

let runDirReady = false;

function ensureRunDir(): void {
  if (!runDirReady && runDir) {
    mkdirSync(runDir, { recursive: true });
    runDirReady = true;
  }
}

export function emitEvent(event: Record<string, unknown>): void {
  if (!runDir) return;
  try {
    ensureRunDir();
    appendFileSync(`${runDir}/events.jsonl`, JSON.stringify(event) + "\n");
  } catch {
    // best-effort
  }
}

export function log(label: string, ...args: unknown[]): void {
  const msg = args.map(String).join(" ");
  const line = `[openflow] ${label}: ${msg}`;
  process.stderr.write(line + "\n");
}

export function deriveAgentName(systemPrompt: string): string {
  const words = systemPrompt
    .toLowerCase()
    .replace(/[^a-z0-9 ]/g, " ")
    .split(/\s+/)
    .filter((w) => w.length > 0)
    .slice(0, 4)
    .join("-")
    .replace(/^-|-$/g, "");

  const hash = createHash("md5")
    .update(systemPrompt)
    .digest("hex")
    .slice(0, 8);

  return `${words}-${hash}`;
}

export function generateSessionID(): string {
  const now = new Date();
  const ts =
    now.getFullYear().toString() +
    String(now.getMonth() + 1).padStart(2, "0") +
    String(now.getDate()).padStart(2, "0") +
    "_" +
    String(now.getHours()).padStart(2, "0") +
    String(now.getMinutes()).padStart(2, "0") +
    String(now.getSeconds()).padStart(2, "0");
  const suffix = randomBytes(4).toString("hex");
  return `openflow_session_${ts}_${suffix}`;
}
