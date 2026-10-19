import { aoBridge } from "./bridge";
import { setApiBaseUrl, setApiDaemonStatus } from "./api-client";
import { usesLiveWebDaemon } from "./preview-mode";

export type DaemonStatus = Awaited<ReturnType<typeof aoBridge.daemon.getStatus>>;

export function applyDaemonStatus(nextStatus: DaemonStatus): void {
	setApiDaemonStatus(nextStatus);
	if (nextStatus.state === "ready" && nextStatus.port) {
		setApiBaseUrl(usesLiveWebDaemon ? window.location.origin : `http://127.0.0.1:${nextStatus.port}`);
	} else {
		setApiBaseUrl(null);
	}
}

export async function refreshDaemonStatus(): Promise<DaemonStatus> {
	const nextStatus = await readDaemonStatus();
	applyDaemonStatus(nextStatus);
	return nextStatus;
}

export async function readDaemonStatus(): Promise<DaemonStatus> {
	if (usesLiveWebDaemon) {
		try {
			const response = await fetch("/readyz", { cache: "no-store" });
			const health = await response.json();
			if (!response.ok || health.status !== "ready") return { state: "stopped" };
			const origin = new URL(window.location.origin);
			// This is the same-origin API proxy's connection port, not the
			// daemon's private bind port behind that proxy.
			const port = Number(origin.port || (origin.protocol === "https:" ? "443" : "80"));
			return { state: "ready", port, pid: health.pid };
		} catch { return { state: "stopped" }; }
	}
	return aoBridge.daemon.getStatus();
}
