// The opt-in pilot web client uses the real same-origin daemon proxy. Ordinary
// dev:web screenshots/e2e retain their existing fixture mode.
export const usesLiveWebDaemon = import.meta.env.VITE_AO_LIVE_WEB === "1";
export const usesPreviewWorkspaceData = import.meta.env.VITE_NO_ELECTRON === "1" && !usesLiveWebDaemon;
