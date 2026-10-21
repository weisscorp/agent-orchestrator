package store_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

func TestWorkspaceScopeIsDurableAndImmutable(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "workspace")
	rec := sampleRecord("workspace")
	rec.Metadata.WorkspaceScopeVersion = 1
	rec.Metadata.WorkspaceScope = &domain.WorkspaceScope{Version: 1, RootPath: "/tmp/workspace", Repos: []domain.WorkspaceScopeRepo{{Name: "api", RelativePath: "api"}}}
	created, err := s.CreateSession(context.Background(), rec)
	if err != nil {
		t.Fatal(err)
	}
	loaded, found, err := s.GetSession(context.Background(), created.ID)
	if err != nil || !found {
		t.Fatalf("get: %v %v", found, err)
	}
	if loaded.Metadata.WorkspaceScopeVersion != 1 || !reflect.DeepEqual(loaded.Metadata.WorkspaceScope, rec.Metadata.WorkspaceScope) {
		t.Fatalf("scope not durable: %+v", loaded.Metadata)
	}
	loaded.Metadata.WorkspaceScope = nil
	loaded.Metadata.WorkspaceScopeVersion = 0
	if err := s.UpdateSession(context.Background(), loaded); err != nil {
		t.Fatal(err)
	}
	loaded, _, err = s.GetSession(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Metadata.WorkspaceScope, rec.Metadata.WorkspaceScope) {
		t.Fatal("generic UpdateSession changed immutable scope")
	}
	legacy, err := s.CreateSession(context.Background(), sampleRecord("workspace"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, _, err = s.GetSession(context.Background(), legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Metadata.WorkspaceScopeVersion != 0 || loaded.Metadata.WorkspaceScope != nil {
		t.Fatal("legacy NULL scope became a selection")
	}
}
