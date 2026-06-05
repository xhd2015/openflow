import { Agent, shell, step } from "openflow";

const agent = new Agent({
  systemPrompt: "You are a helpful assistant. Keep responses short.",
});

let result = "";

await step("GREET", async () => {
  result = await agent.run("Say 'hello from openflow' and nothing else");
  console.log("agent said:", result.trim());
});

await step("VERIFY", async () => {
  const { stdout } = await shell("echo 'done'");
  console.log(stdout.trim());
});

if (result.toLowerCase().includes("hello")) {
  console.log("SUCCESS: workflow completed");
} else {
  console.log("UNEXPECTED: agent did not say hello");
  process.exit(1);
}
