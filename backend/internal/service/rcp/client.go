// Package rcp reads development context from an independently managed RCP.
// It never creates features, launches work or changes delivery state.
package rcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// Feature is the development scope and requirements returned by RCP.
type Feature struct {
	ID           string   `json:"id"`
	ProductID    string   `json:"product_id"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	Goal         string   `json:"goal"`
	Problem      string   `json:"problem"`
	Requirements []string `json:"requirements"`
	Constraints  []string `json:"constraints"`
	Repositories []string `json:"repositories"`
}

// Integration describes one implementation effort within a feature.
type Integration struct {
	ID                 string   `json:"id"`
	FeatureID          string   `json:"feature_id"`
	Title              string   `json:"title"`
	Objective          string   `json:"objective"`
	Status             string   `json:"status"`
	Owner              string   `json:"owner"`
	Repositories       []string `json:"repositories"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Completed          []string `json:"completed"`
	Remaining          []string `json:"remaining"`
}

// Repository identifies a repository registered in RCP.
type Repository struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Memory contains progress or handoff context returned by RCP.
type Memory struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	IntegrationID string   `json:"integration_id"`
	Body          string   `json:"body"`
	CreatedAt     string   `json:"created_at"`
	Title         string   `json:"title"`
	Reason        string   `json:"reason"`
	Current       string   `json:"current"`
	Completed     []string `json:"completed"`
	Remaining     []string `json:"remaining"`
	Next          []string `json:"next"`
	Warnings      []string `json:"warnings"`
}

// Catalog is the bounded feature list and repository catalog.
type Catalog struct {
	Features     []Feature    `json:"features"`
	Repositories []Repository `json:"repositories"`
	Truncated    bool         `json:"truncated"`
}

// Context contains a feature and its bounded development history.
type Context struct {
	Feature          Feature       `json:"feature"`
	Integrations     []Integration `json:"integrations"`
	Repositories     []Repository  `json:"repositories"`
	Memories         []Memory      `json:"memories"`
	HistoryTotal     int           `json:"history_total"`
	HistoryTruncated bool          `json:"history_truncated"`
}

// Client reads development context from a configured RCP endpoint.
type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

// New validates the endpoint and constructs a read-only RCP client.
func New(address, token string) (*Client, error) {
	u, err := url.Parse(address)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("invalid RCP address")
	}
	ip := net.ParseIP(u.Hostname())
	loopback := u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()
	if u.Scheme != "https" && (u.Scheme != "http" || !loopback) {
		return nil, errors.New("RCP requires HTTPS or local loopback HTTP")
	}
	return &Client{base: u, token: token, http: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) read(ctx context.Context, endpoint string, limit int64, value any) error {
	u := *c.base
	u.Path = endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return errors.New("invalid RCP request")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return errors.New("RCP unavailable; check its address and connection")
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("RCP returned HTTP %d; check access and feature selection", res.StatusCode)
	}
	reader := &io.LimitedReader{R: res.Body, N: limit + 1}
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(value); err != nil {
		return errors.New("RCP returned invalid or oversized context")
	}
	if decoder.Decode(new(any)) != io.EOF || reader.N <= 0 {
		return errors.New("RCP returned invalid or oversized context")
	}
	return nil
}

// List returns a bounded catalog, optionally filtered by product. The current
// RCP has no paginated feature endpoint. Decode only these fields,
// never materialize its large event/log/credential arrays, and enforce a hard
// upstream response bound. Call only on initial load or explicit refresh.
func (c *Client) List(ctx context.Context, product string) (Catalog, error) {
	var state struct {
		Features     []Feature    `json:"features"`
		Repositories []Repository `json:"repositories"`
	}
	if err := c.read(ctx, "/api/state", 128<<20, &state); err != nil {
		return Catalog{}, err
	}
	out := Catalog{Features: []Feature{}, Repositories: state.Repositories}
	for _, f := range state.Features {
		if (product != "" && f.ProductID != product) || f.Status == "archived" {
			continue
		}
		if len(out.Features) == 200 {
			out.Truncated = true
			continue
		}
		f.Goal = ""
		f.Problem = ""
		f.Requirements = nil
		f.Constraints = nil
		out.Features = append(out.Features, f)
	}
	return out, nil
}

var validID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// Resume reads one feature's context and the most recent history entries.
func (c *Client) Resume(ctx context.Context, id string) (Context, error) {
	if !validID.MatchString(id) {
		return Context{}, errors.New("invalid RCP feature id")
	}
	var out Context
	if err := c.read(ctx, "/api/features/"+id+"/context", 8<<20, &out); err != nil {
		return Context{}, err
	}
	if out.Feature.ID != id {
		return Context{}, errors.New("RCP returned a different feature")
	}
	out.HistoryTotal = len(out.Memories)
	if len(out.Memories) > 20 {
		out.Memories = out.Memories[len(out.Memories)-20:]
		out.HistoryTruncated = true
	}
	return out, nil
}
