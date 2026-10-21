package sessionmanager

import (
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

func TestRCPMCPDefaultsToLocalAndIsDaemonScoped(t *testing.T) {
	mgr, _, _ := newChatManager(&recordingLauncher{})
	mgr.executable = func() (string, error) { return "/opt/ao/bin/ao", nil }
	t.Setenv("AO_RCP_URL", "")
	t.Setenv("AO_RCP_TOKEN", "")
	if got := mgr.aoMCPServers(domain.HarnessCodex, map[string]string{"AO_RCP_URL": "http://evil.example", "AO_RCP_TOKEN": "secret"}); len(got) != 2 || got[1].URL != "http://127.0.0.1:8090/mcp" || len(got[1].Headers) != 0 {
		t.Fatalf("repository env changed the credential-free local default: %+v", got)
	}
	if got := mgr.aoMCPServers(domain.HarnessQwen, nil); len(got) != 1 {
		t.Fatalf("default HTTP MCP injected into an ACP harness without known capability: %+v", got)
	}
	t.Setenv("AO_RCP_URL", "http://127.0.0.1:8090")
	got := mgr.aoMCPServers(domain.HarnessCodex, nil)
	if len(got) != 2 || got[1].Name != "release_control" || got[1].URL != "http://127.0.0.1:8090/mcp" || len(got[1].Headers) > 0 {
		t.Fatalf("missing or unsafe MCP: %+v", got)
	}
	t.Setenv("AO_RCP_TOKEN", "secret")
	if len(mgr.aoMCPServers(domain.HarnessCodex, nil)) != 1 {
		t.Fatal("credential-bearing MCP must not be serialized")
	}
	t.Setenv("AO_RCP_TOKEN", "")
	t.Setenv("AO_RCP_URL", "http://evil.example")
	if len(mgr.aoMCPServers(domain.HarnessCodex, nil)) != 1 {
		t.Fatal("non-pilot address enabled")
	}
}
