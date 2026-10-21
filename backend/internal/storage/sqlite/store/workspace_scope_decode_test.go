package store

import (
	"database/sql"
	"testing"
)

func TestDecodeWorkspaceScopePreservesCorruptionMarker(t *testing.T) {
	if scope, version := decodeWorkspaceScope(sql.NullString{}); scope != nil || version != 0 {
		t.Fatal("NULL must retain legacy identity")
	}
	for _, raw := range []string{"", "invalid", "null", `{}`, `{"version":99,"rootPath":"/root","repos":[]}`} {
		scope, version := decodeWorkspaceScope(sql.NullString{Valid: true, String: raw})
		if scope == nil && version == 0 {
			t.Fatalf("corrupt scope %q collapsed to legacy", raw)
		}
	}
}
