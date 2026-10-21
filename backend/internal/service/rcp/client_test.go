package rcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRCPScopedContextAndAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("never expose upstream secrets"))
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected write: %s", r.Method)
		}
		switch r.URL.Path {
		case "/api/state":
			_ = json.NewEncoder(w).Encode(map[string]any{"features": []Feature{{ID: "f", ProductID: "p", Title: "Two repositories", Status: "planned", Repositories: []string{"back", "front"}}, {ID: "other", ProductID: "q"}, {ID: "old", ProductID: "p", Status: "archived"}}, "events": []string{strings.Repeat("irrelevant", 1000)}, "repositories": []Repository{{ID: "back", Name: "backend"}, {ID: "front", Name: "frontend"}}})
		case "/api/features/f/context":
			memories := make([]Memory, 25)
			for i := range memories {
				memories[i] = Memory{ID: "memory", Current: "progress", Completed: []string{"tested"}}
			}
			_ = json.NewEncoder(w).Encode(Context{Feature: Feature{ID: "f", Repositories: []string{"back", "front"}}, Integrations: []Integration{{ID: "i", FeatureID: "f", Repositories: []string{"back"}}}, Memories: memories})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := New(server.URL, "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := client.List(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Features) != 1 || len(catalog.Features[0].Repositories) != 2 || len(catalog.Repositories) != 2 {
		t.Fatalf("wrong catalog: %+v", catalog)
	}
	result, err := client.Resume(context.Background(), "f")
	if err != nil {
		t.Fatal(err)
	}
	if result.Feature.ID != "f" || len(result.Memories) != 20 || !result.HistoryTruncated || result.HistoryTotal != 25 || result.Memories[0].Completed[0] != "tested" {
		t.Fatalf("wrong context: %+v", result)
	}
	if _, err := client.Resume(context.Background(), "../state"); err == nil {
		t.Fatal("accepted traversal")
	}
	unauth, _ := New(server.URL, "")
	_, err = unauth.List(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "secrets") {
		t.Fatalf("unsafe auth error: %v", err)
	}
}

func TestRCPRejectsUnsafeResponsesAndAddresses(t *testing.T) {
	for _, address := range []string{"http://public.example", "file:///etc/passwd", "https://token@example.com", "http://127.0.0.1/path", "http://127.0.0.1?token=secret"} {
		if _, err := New(address, ""); err == nil {
			t.Errorf("accepted %s", address)
		}
	}
	for _, body := range []string{`{"feature":{"id":"wrong"}}`, `{"feature":{"id":"f"}} {}`, `not json`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		client, _ := New(server.URL, "")
		if _, err := client.Resume(context.Background(), "f"); err == nil {
			t.Errorf("accepted invalid response %s", body)
		}
		server.Close()
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"feature":{"id":"f"}}`)) }))
	defer server.Close()
	client, _ := New(server.URL, "")
	var value Context
	if err := client.read(context.Background(), "/", 4, &value); err == nil {
		t.Fatal("accepted oversized response")
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, server.URL, http.StatusFound) }))
	defer redirect.Close()
	client, _ = New(redirect.URL, "secret")
	if _, err := client.Resume(context.Background(), "f"); err == nil {
		t.Fatal("followed credential-bearing redirect")
	}
}
