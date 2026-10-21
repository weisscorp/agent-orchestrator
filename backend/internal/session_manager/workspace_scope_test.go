package sessionmanager

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestSpawnWorkspaceRepositorySelection(t *testing.T) {
	for _, tc := range []struct {
		name           string
		selected, want []string
	}{
		{"omitted", nil, []string{"api", "broken"}},
		{"root only", []string{}, []string{}},
		{"selected", []string{"api"}, []string{"api"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, st, _, ws := newManager()
			st.projects["mer"] = domain.ProjectRecord{ID: "mer", Path: "/repo/workspace", Kind: domain.ProjectKindWorkspace, Config: testRoleAgents()}
			st.workspaceRepo["mer"] = []domain.WorkspaceRepoRecord{{Name: "api", RelativePath: "api"}, {Name: "broken", RelativePath: "broken"}}
			rec, _, _, err := m.Spawn(context.Background(), ports.SpawnConfig{ProjectID: "mer", Kind: domain.KindWorker, WorkspaceRepos: tc.selected})
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0)
			for _, repo := range ws.lastProjectCfg.Repos {
				got = append(got, repo.Name)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("created=%v want=%v", got, tc.want)
			}
			if len(ws.resolves) != 1+len(tc.want) || len(ws.fetches) != 1+len(tc.want) {
				t.Fatalf("probed unrelated repos: resolves=%v fetches=%v", ws.resolves, ws.fetches)
			}
			if rec.Metadata.WorkspaceScopeVersion != 1 || rec.Metadata.WorkspaceScope == nil || len(rec.Metadata.WorkspaceScope.Repos) != len(tc.want) {
				t.Fatalf("missing durable scope: %+v", rec.Metadata)
			}
		})
	}
}

func TestInvalidWorkspaceSelectionDoesNotTouchGit(t *testing.T) {
	for _, selected := range [][]string{{"unknown"}, {"api", "api"}, {"__root__"}, {" api"}, {""}} {
		m, st, _, ws := newManager()
		st.projects["mer"] = domain.ProjectRecord{ID: "mer", Path: "/repo/root", Kind: domain.ProjectKindWorkspace, Config: testRoleAgents()}
		st.workspaceRepo["mer"] = []domain.WorkspaceRepoRecord{{Name: "api", RelativePath: "api"}}
		if _, _, _, err := m.Spawn(context.Background(), ports.SpawnConfig{ProjectID: "mer", Kind: domain.KindWorker, WorkspaceRepos: selected}); err == nil {
			t.Fatalf("accepted %v", selected)
		}
		if len(ws.resolves) != 0 || ws.createCount != 0 || len(st.sessions) != 0 {
			t.Fatalf("Git/session mutation for %v", selected)
		}
	}
	m, _, _, ws := newManager()
	if _, _, _, err := m.Spawn(context.Background(), ports.SpawnConfig{ProjectID: "mer", Kind: domain.KindWorker, WorkspaceRepos: []string{}}); err == nil {
		t.Fatal("accepted nonworkspace selection")
	}
	if len(ws.resolves) != 0 {
		t.Fatal("nonworkspace selection touched Git")
	}
}

func TestWorkspaceScopeSurvivesRestartAndRegistryChanges(t *testing.T) {
	m, st, _, _ := newManager()
	project := domain.ProjectRecord{ID: "mer", Path: "/repo/root", Kind: domain.ProjectKindWorkspace}
	st.workspaceRepo["mer"] = []domain.WorkspaceRepoRecord{{Name: "api", RelativePath: "services/api", RepoOriginURL: "https://example.com/api.git"}}
	scope, err := m.resolveWorkspaceScope(context.Background(), project, []string{"api"})
	if err != nil {
		t.Fatal(err)
	}
	rec := domain.SessionRecord{ID: "mer-1", ProjectID: "mer", Metadata: domain.SessionMetadata{WorkspaceScope: scope, WorkspaceScopeVersion: 1, WorkspacePath: "/ws/mer-1"}}
	data, err := json.Marshal(rec.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	loaded := rec
	if err := json.Unmarshal(data, &loaded.Metadata); err != nil {
		t.Fatal(err)
	}
	st.workspaceRepo["mer"] = []domain.WorkspaceRepoRecord{{Name: "api", RelativePath: "replacement"}, {Name: "new", RelativePath: "new"}}
	rows := []domain.SessionWorktreeRecord{{RepoName: domain.RootWorkspaceRepoName, WorktreePath: "/ws/mer-1"}, {RepoName: "api", WorktreePath: "/ws/mer-1/services/api"}}
	infos, err := m.workspaceProjectRestoreRowsFromMarkers(context.Background(), project, loaded, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 || infos[1].RepoPath != filepath.Join(project.Path, "services/api") {
		t.Fatalf("scope widened or retargeted: %+v", infos)
	}
	if len(scope.Repos) != 1 || strings.Contains(string(data), "example.com") {
		t.Fatal("persisted raw origin")
	}
	for _, bad := range []*domain.WorkspaceScope{nil, {Version: 2, RootPath: project.Path, Repos: []domain.WorkspaceScopeRepo{}}, {Version: 1, RootPath: project.Path}} {
		loaded.Metadata.WorkspaceScope = bad
		if _, err := m.workspaceProjectRestoreRowsFromMarkers(context.Background(), project, loaded, rows); err == nil {
			t.Fatalf("corrupt scope accepted: %+v", bad)
		}
	}
}

func TestWorkspaceTaskPreparationWaitsForSelection(t *testing.T) {
	m, st, _, ws := newManager()
	project := st.projects["mer"]
	project.Kind = domain.ProjectKindWorkspace
	token, err := m.PrepareTaskWorkspace(context.Background(), project)
	if err != nil || token != "" || len(st.sessions) != 0 || len(ws.resolves) != 0 {
		t.Fatalf("unscoped warmup: token=%q err=%v", token, err)
	}
}

func TestRetryWorkspaceScopeKeepsPinnedSelection(t *testing.T) {
	m, st, _, _ := newManager()
	project := domain.ProjectRecord{ID: "mer", Path: "/repo/root", Kind: domain.ProjectKindWorkspace}
	for _, selected := range [][]string{{}, {"api"}} {
		st.workspaceRepo["mer"] = []domain.WorkspaceRepoRecord{{Name: "api", RelativePath: "api"}}
		scope, err := m.resolveWorkspaceScope(context.Background(), project, selected)
		if err != nil {
			t.Fatal(err)
		}
		rec := domain.SessionRecord{ID: "mer-1", ProjectID: "mer", Metadata: domain.SessionMetadata{WorkspaceScope: scope, WorkspaceScopeVersion: 1}}
		st.workspaceRepo["mer"] = []domain.WorkspaceRepoRecord{{Name: "api", RelativePath: "replacement"}, {Name: "unrelated", RelativePath: "unrelated"}}
		got, err := m.retryWorkspaceScope(context.Background(), project, rec)
		if err != nil || !reflect.DeepEqual(got, scope) {
			t.Fatalf("retry selection changed: got=%+v err=%v", got, err)
		}
	}
	legacy := domain.SessionRecord{ID: "legacy", ProjectID: "mer"}
	if _, err := m.retryWorkspaceScope(context.Background(), project, legacy); err == nil {
		t.Fatal("legacy retry without inventory expanded selection")
	}
}

func TestWorkspaceScopeMissingOwnershipRowsPreservesWorkspace(t *testing.T) {
	m, st, _, ws := newManager()
	st.projects["mer"] = domain.ProjectRecord{ID: "mer", Path: "/repo/root", Kind: domain.ProjectKindWorkspace}
	rec := domain.SessionRecord{ID: "mer-1", ProjectID: "mer", Metadata: domain.SessionMetadata{
		WorkspacePath: "/ws/mer-1", WorkspaceScopeVersion: 1,
		WorkspaceScope: &domain.WorkspaceScope{Version: 1, RootPath: "/repo/root", Repos: []domain.WorkspaceScopeRepo{{Name: "api", RelativePath: "api"}}},
	}}
	if _, _, err := m.workspaceProjectRows(context.Background(), rec); err == nil {
		t.Fatal("missing inventory accepted")
	}
	st.worktrees[rec.ID] = []domain.SessionWorktreeRecord{{RepoName: domain.RootWorkspaceRepoName, WorktreePath: "/ws/mer-1"}}
	if _, _, err := m.workspaceProjectRows(context.Background(), rec); err == nil {
		t.Fatal("missing selected child row accepted")
	}
	if ws.destroyed != 0 {
		t.Fatal("incomplete inventory destroyed a workspace")
	}
}

func TestLegacyWorkspaceRestoreUsesOnlyRecordedRows(t *testing.T) {
	m, st, _, _ := newManager()
	project := domain.ProjectRecord{ID: "mer", Path: "/repo/root", Kind: domain.ProjectKindWorkspace}
	st.workspaceRepo["mer"] = []domain.WorkspaceRepoRecord{{Name: "new", RelativePath: "new"}}
	rec := domain.SessionRecord{ID: "mer-1", ProjectID: "mer", Metadata: domain.SessionMetadata{WorkspacePath: "/ws/mer-1"}}
	rows := []domain.SessionWorktreeRecord{{RepoName: domain.RootWorkspaceRepoName, WorktreePath: "/ws/mer-1"}}
	infos, err := m.workspaceProjectRestoreRowsFromMarkers(context.Background(), project, rec, rows)
	if err != nil || len(infos) != 1 {
		t.Fatalf("legacy expanded: %+v %v", infos, err)
	}
	if _, err := m.workspaceProjectRestoreRowsFromMarkers(context.Background(), project, rec, nil); err == nil {
		t.Fatal("guessed missing worktree inventory")
	}
}

func TestWorkspaceSelectionSkipsUnselectedInvalidOriginHEAD(t *testing.T) {
	m, st, _, root := newGitTaskPreparationManager(t)
	child := newManagerGitRepo(t)
	childPath := filepath.Join(root, "broken")
	if err := os.Rename(child, childPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("broken/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runManagerGit(t, root, "add", ".gitignore")
	runManagerGit(t, root, "commit", "-m", "ignore child")
	project := st.projects["mer"]
	project.Kind = domain.ProjectKindWorkspace
	st.projects["mer"] = project
	st.workspaceRepo["mer"] = []domain.WorkspaceRepoRecord{{Name: "broken", RelativePath: "broken", GitStatus: domain.GitStatusReady}}
	rootOnly, err := m.resolveWorkspaceScope(context.Background(), project, []string{})
	if err != nil {
		t.Fatal(err)
	}
	refs := m.refreshDefaultBranchesBestEffort(context.Background(), project, rootOnly)
	_, info, err := m.createSessionWorkspace(context.Background(), project, ports.SpawnConfig{ProjectID: "mer", Kind: domain.KindWorker, WorkspaceScope: rootOnly}, "mer-1", "ao/root-only", refs)
	if err != nil {
		t.Fatalf("root-only start touched invalid child: %v", err)
	}
	if len(info.Worktrees) != 1 {
		t.Fatalf("root-only worktrees: %+v", info.Worktrees)
	}
	selected, err := m.resolveWorkspaceScope(context.Background(), project, []string{"broken"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = m.createSessionWorkspace(context.Background(), project, ports.SpawnConfig{ProjectID: "mer", Kind: domain.KindWorker, WorkspaceScope: selected}, "mer-2", "ao/selected-broken", nil)
	if err == nil || !strings.Contains(err.Error(), "default branch") {
		t.Fatalf("selected invalid origin/HEAD error=%v", err)
	}
}
