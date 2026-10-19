import { afterEach, describe, expect, it, vi } from "vitest";
import { refreshDaemonStatus } from "./daemon-status";

const mocks = vi.hoisted(() => ({
	live: true,
	getStatus: vi.fn(),
	setBase: vi.fn(),
	setStatus: vi.fn(),
}));

vi.mock("./bridge", () => ({ aoBridge: { daemon: { getStatus: mocks.getStatus } } }));
vi.mock("./api-client", () => ({ setApiBaseUrl: mocks.setBase, setApiDaemonStatus: mocks.setStatus }));
vi.mock("./preview-mode", () => ({ get usesLiveWebDaemon() { return mocks.live; } }));

afterEach(() => {
	vi.unstubAllGlobals();
	vi.clearAllMocks();
	mocks.live = true;
});

describe("same-origin daemon proxy status", () => {
	it.each([
		["http://localhost:4321", 4321],
		["http://example.test", 80],
		["https://example.test", 443],
	])("uses the proxy origin %s without assuming a daemon bind port", async (origin, port) => {
		vi.stubGlobal("window", { location: new URL(origin) });
		const fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ status: "ready", pid: 42 }) });
		vi.stubGlobal("fetch", fetch);
		expect(await refreshDaemonStatus()).toEqual({ state: "ready", port, pid: 42 });
		expect(fetch).toHaveBeenCalledWith("/readyz", { cache: "no-store" });
		expect(mocks.setBase).toHaveBeenCalledWith(origin);
		expect(mocks.getStatus).not.toHaveBeenCalled();
	});

	it("keeps native daemon status and its loopback endpoint unchanged", async () => {
		mocks.live = false;
		mocks.getStatus.mockResolvedValue({ state: "ready", port: 3001, pid: 42 });
		expect(await refreshDaemonStatus()).toEqual({ state: "ready", port: 3001, pid: 42 });
		expect(mocks.setBase).toHaveBeenCalledWith("http://127.0.0.1:3001");
	});
});
