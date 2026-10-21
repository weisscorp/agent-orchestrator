import { app, dialog, shell } from "electron";
import semver from "semver";
import { readFileSync } from "node:fs";
import path from "node:path";

// Read the same build-time repository identity as electron-updater. Never
// redirect a fork to upstream when its release configuration is unavailable.
function releaseRepository(): string | null {
  try {
    const yml = readFileSync(path.join(process.resourcesPath, "app-update.yml"), "utf8");
    const owner = /^owner:\s*(.+)$/m.exec(yml)?.[1]?.trim();
    const repo = /^repo:\s*(.+)$/m.exec(yml)?.[1]?.trim();
    return owner && repo && /^[A-Za-z0-9_-]+$/.test(owner) && /^[A-Za-z0-9_.-]+$/.test(repo)
      && repo !== "." && repo !== ".." ? `${owner}/${repo}` : null;
  } catch {
    return null;
  }
}

const FETCH_TIMEOUT_MS = 10_000;

type Floor = {
  min?: string;
  latest?: string;
  downloadUrl?: string;
};


function usableVersion(v: string | null | undefined): string | null {
  const trimmed = v?.trim();
  return trimmed && /^\d{1,4}(\.\d{1,4}){0,3}(-[a-zA-Z0-9.]+)?$/.test(trimmed)
    ? trimmed
    : null;
}

function isBelow(running: string, floor: string): boolean {
  const r = semver.valid(semver.coerce(running));
  const f = semver.valid(semver.coerce(floor));
  if (!r || !f) return false;
  return semver.lt(r, f);
}

export async function checkDesktopVersionFloor(): Promise<void> {
  if (!app.isPackaged) return;
  const repo = releaseRepository();
  if (!repo) return;
  const floorUrl = `https://raw.githubusercontent.com/${repo}/main/desktop-version-floor.json`;
  const defaultDownloadUrl = `https://github.com/${repo}/releases/latest`;
  let floor: Floor;
  try {
    const response = await fetch(floorUrl, {
      cache: "no-store",
      headers: {
        "User-Agent": `ao-desktop/${app.getVersion()}`,
        Accept: "application/json",
      },
      signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
    });
    if (!response.ok) return;
    floor = (await response.json()) as Floor;
  } catch {
    return;
  }

  const running = app.getVersion();
  const rawUrl = floor.downloadUrl || defaultDownloadUrl;
  const downloadUrl = rawUrl.startsWith(`https://github.com/${repo}/releases/`) ? rawUrl : defaultDownloadUrl;
  const minVersion = usableVersion(floor.min);
  const latestVersion = usableVersion(floor.latest);

  if (minVersion && isBelow(running, minVersion)) {
    const result = await dialog.showMessageBox({
      type: "warning",
      buttons: ["Download Update", "Quit"],
      defaultId: 0,
      cancelId: 1,
      title: "Update Required",
      message: `Agent Orchestrator v${running} is no longer supported.`,
      detail:
        `Version ${minVersion} or later is required. ` +
        "Download the latest release to continue using the app.",
      noLink: true,
    });
    if (result.response === 0) {
      void shell.openExternal(downloadUrl);
    }
    app.quit();
    return;
  }

  if (latestVersion && isBelow(running, latestVersion)) {
    const result = await dialog.showMessageBox({
      type: "info",
      buttons: ["Download Update", "Later"],
      defaultId: 0,
      cancelId: 1,
      title: "Update Available",
      message: "A newer version of Agent Orchestrator is available.",
      detail:
        `You are on v${running}. Version ${latestVersion} is recommended. ` +
        "You can update now or continue using the current version.",
      noLink: true,
    });
    if (result.response === 0) {
      void shell.openExternal(downloadUrl);
    }
  }
}
