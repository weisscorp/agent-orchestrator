import { spawnSync } from "node:child_process";
import { realpathSync } from "node:fs";
import path from "node:path";

// Forge's unsigned arm64 fuse pass ad-hoc signs before Packager updates the
// final Info.plist. Seal only this local build output after packaging completes.
// This is structural local signing, never Developer ID or notarization.
if (process.platform !== "darwin" || process.arch !== "arm64") {
  throw new Error("Local macOS sealing requires a native arm64 macOS build");
}
if (["APPLE_SIGNING_IDENTITY", "CSC_LINK", "AO_NOTARY_PROFILE", "APPLE_API_KEY"].some(key => process.env[key])) {
  throw new Error("Use canonical signing for credential-configured builds, not local ad-hoc sealing");
}
const build = spawnSync("npm", ["run", "package"], { stdio: "inherit" });
if (build.error) throw build.error;
if (build.status !== 0) throw new Error(`Local app packaging failed (${build.status ?? build.signal})`);
const app = realpathSync(path.resolve("out/Agent Orchestrator-darwin-arm64/Agent Orchestrator.app"));
const expected = path.resolve("out/Agent Orchestrator-darwin-arm64/Agent Orchestrator.app");
if (app !== expected) throw new Error("Refusing to sign a redirected build output");
const signature = spawnSync("/usr/bin/codesign", ["--display", "--verbose=2", app], { encoding: "utf8" });
if (signature.status !== 0 || !signature.stderr.includes("Signature=adhoc")) {
  throw new Error("Refusing to replace a non-ad-hoc app signature");
}
for (const args of [
  ["--force", "--deep", "--sign", "-", "--preserve-metadata=entitlements,requirements,flags,runtime", app],
  ["--verify", "--deep", "--strict", "--verbose=2", app],
]) {
  const result = spawnSync("/usr/bin/codesign", args, { stdio: "inherit" });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`Local app sealing failed (${result.status ?? result.signal})`);
}
console.log(`Local ad-hoc app sealed: ${app}\nNot notarized; no system security settings changed.`);
