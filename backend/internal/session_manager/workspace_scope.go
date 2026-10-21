package sessionmanager

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

// ErrInvalidWorkspaceSelection identifies invalid workspace repository choices.
var ErrInvalidWorkspaceSelection = errors.New("invalid workspace repository selection")

func firstWorkspaceScope(scopes []*domain.WorkspaceScope) *domain.WorkspaceScope {
	if len(scopes) == 0 {
		return nil
	}
	return scopes[0]
}

func validateWorkspaceScope(project domain.ProjectRecord, scope *domain.WorkspaceScope) error {
	if scope == nil {
		return nil
	}
	if scope.Version != 1 || scope.Repos == nil || scope.RootPath == "" || filepath.Clean(scope.RootPath) != filepath.Clean(project.Path) || project.Kind.WithDefault() != domain.ProjectKindWorkspace {
		return fmt.Errorf("invalid persisted workspace repository scope")
	}
	names := map[string]bool{}
	paths := map[string]bool{}
	for _, repo := range scope.Repos {
		path := filepath.FromSlash(repo.RelativePath)
		if repo.Name == "" || repo.Name == "." || repo.Name == ".." || strings.ContainsAny(repo.Name, "/\\") || strings.IndexFunc(repo.Name, unicode.IsControl) >= 0 || repo.Name == domain.RootWorkspaceRepoName || strings.TrimSpace(repo.Name) != repo.Name || names[repo.Name] || path == "." || path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) || paths[path] || path == ".git" || strings.HasPrefix(path, ".git"+string(filepath.Separator)) || (repo.GitStatus != "" && repo.GitStatus != domain.GitStatusReady && repo.GitStatus != domain.GitStatusNeedsInit) {
			return fmt.Errorf("invalid persisted workspace repository scope for %q", repo.Name)
		}
		names[repo.Name] = true
		paths[path] = true
	}
	return nil
}

func (m *Manager) resolveWorkspaceScope(ctx context.Context, project domain.ProjectRecord, selected []string) (*domain.WorkspaceScope, error) {
	if project.Kind.WithDefault() != domain.ProjectKindWorkspace {
		if selected != nil {
			return nil, fmt.Errorf("%w: workspaceRepos requires a workspace project", ErrInvalidWorkspaceSelection)
		}
		return nil, nil
	}
	repos, err := m.store.ListWorkspaceRepos(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	return ResolveWorkspaceScope(project, repos, selected)
}

// ResolveWorkspaceScope validates names without inspecting or changing Git.
func ResolveWorkspaceScope(project domain.ProjectRecord, repos []domain.WorkspaceRepoRecord, selected []string) (*domain.WorkspaceScope, error) {
	if project.Kind.WithDefault() != domain.ProjectKindWorkspace {
		if selected != nil {
			return nil, fmt.Errorf("%w: workspaceRepos requires a workspace project", ErrInvalidWorkspaceSelection)
		}
		return nil, nil
	}
	scope := &domain.WorkspaceScope{Version: 1, RootPath: project.Path, Repos: make([]domain.WorkspaceScopeRepo, 0)}
	byName := make(map[string]domain.WorkspaceRepoRecord, len(repos))
	for _, repo := range repos {
		byName[repo.Name] = repo
	}
	if selected == nil {
		selected = make([]string, 0, len(repos))
		for _, repo := range repos {
			selected = append(selected, repo.Name)
		}
	}
	seen := map[string]bool{}
	for _, name := range selected {
		repo, ok := byName[name]
		if !ok || name == "" || name == domain.RootWorkspaceRepoName || strings.TrimSpace(name) != name || seen[name] {
			return nil, fmt.Errorf("%w: invalid, unknown or duplicate workspace repository %q", ErrInvalidWorkspaceSelection, name)
		}
		seen[name] = true
		scope.Repos = append(scope.Repos, domain.WorkspaceScopeRepo{Name: repo.Name, RelativePath: repo.RelativePath, DefaultBranch: repo.DefaultBranch, GitStatus: repo.GitStatus})
	}
	if err := validateWorkspaceScope(project, scope); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidWorkspaceSelection, err)
	}
	return scope, nil
}

func (m *Manager) workspaceScopeRepos(ctx context.Context, project domain.ProjectRecord, scope *domain.WorkspaceScope) ([]domain.WorkspaceRepoRecord, error) {
	if scope == nil {
		return m.store.ListWorkspaceRepos(ctx, project.ID)
	}
	if err := validateWorkspaceScope(project, scope); err != nil {
		return nil, err
	}
	repos := make([]domain.WorkspaceRepoRecord, 0, len(scope.Repos))
	for _, repo := range scope.Repos {
		repos = append(repos, domain.WorkspaceRepoRecord{Name: repo.Name, RelativePath: repo.RelativePath, DefaultBranch: repo.DefaultBranch, GitStatus: repo.GitStatus})
	}
	return repos, nil
}

func workspaceScopeVersion(scope *domain.WorkspaceScope) int {
	if scope == nil {
		return 0
	}
	return 1
}

func (m *Manager) sessionScopeRepos(ctx context.Context, project domain.ProjectRecord, rec domain.SessionRecord) ([]domain.WorkspaceRepoRecord, error) {
	if rec.Metadata.WorkspaceScopeVersion != 0 && (rec.Metadata.WorkspaceScopeVersion != 1 || rec.Metadata.WorkspaceScope == nil) {
		return nil, fmt.Errorf("invalid persisted workspace repository scope")
	}
	return m.workspaceScopeRepos(ctx, project, rec.Metadata.WorkspaceScope)
}

// Legacy retries may recover only repositories with durable ownership rows.
// An absent inventory must never turn a retry into a fresh all-repository spawn.
func (m *Manager) retryWorkspaceScope(ctx context.Context, project domain.ProjectRecord, rec domain.SessionRecord) (*domain.WorkspaceScope, error) {
	if _, err := m.sessionScopeRepos(ctx, project, rec); err != nil {
		return nil, err
	}
	if project.Kind.WithDefault() != domain.ProjectKindWorkspace || rec.Metadata.WorkspaceScope != nil {
		return rec.Metadata.WorkspaceScope, nil
	}
	rows, err := m.store.ListSessionWorktrees(ctx, rec.ID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("legacy workspace session %s has no durable repository inventory", rec.ID)
	}
	selected := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.RepoName != domain.RootWorkspaceRepoName {
			selected = append(selected, row.RepoName)
		}
	}
	scope, err := m.resolveWorkspaceScope(ctx, project, selected)
	if err != nil {
		return nil, err
	}
	check := rec
	check.Metadata.WorkspaceScope = scope
	check.Metadata.WorkspaceScopeVersion = 1
	if _, err := m.sessionWorktreeRowsToRepoInfos(ctx, project, check, rows); err != nil {
		return nil, err
	}
	return scope, nil
}
