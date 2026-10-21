package controllers

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/envelope"
	rcpsvc "github.com/aoagents/agent-orchestrator/backend/internal/service/rcp"
)

// RCPController is a read-only bridge. Configuration and credentials stay in
// the daemon process, never in renderer storage or API parameters.
type RCPController struct{}

// Register mounts the read-only RCP routes.
func (c *RCPController) Register(r chi.Router) {
	r.Get("/rcp/features", c.list)
	r.Get("/rcp/features/{featureId}", c.resume)
}

// RCPFeaturesResponse is the feature catalog returned by the bridge.
type RCPFeaturesResponse = rcpsvc.Catalog

// RCPContextResponse is the context of the selected feature.
type RCPContextResponse = rcpsvc.Context

// RCPFeatureParam selects the feature whose context is requested.
type RCPFeatureParam struct {
	FeatureID string `path:"featureId"`
}

// RCPFeaturesQuery optionally limits the catalog to one product.
type RCPFeaturesQuery struct {
	ProductID string `query:"productId,omitempty"`
}

func rcpClient() (*rcpsvc.Client, error) {
	address := os.Getenv("AO_RCP_URL")
	if address == "" {
		address = "http://127.0.0.1:8090"
	}
	return rcpsvc.New(address, os.Getenv("AO_RCP_TOKEN"))
}
func (c *RCPController) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	client, err := rcpClient()
	if err != nil {
		envelope.WriteAPIError(w, r, 503, "unavailable", "RCP_UNAVAILABLE", err.Error(), nil)
		return
	}
	value, err := client.List(r.Context(), r.URL.Query().Get("productId"))
	if err != nil {
		envelope.WriteAPIError(w, r, 502, "unavailable", "RCP_UNAVAILABLE", err.Error(), nil)
		return
	}
	envelope.WriteJSON(w, 200, value)
}
func (c *RCPController) resume(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	client, err := rcpClient()
	if err != nil {
		envelope.WriteAPIError(w, r, 503, "unavailable", "RCP_UNAVAILABLE", err.Error(), nil)
		return
	}
	value, err := client.Resume(r.Context(), chi.URLParam(r, "featureId"))
	if err != nil {
		envelope.WriteAPIError(w, r, 502, "unavailable", "RCP_UNAVAILABLE", err.Error(), nil)
		return
	}
	envelope.WriteJSON(w, 200, value)
}
