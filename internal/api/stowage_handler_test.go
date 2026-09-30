package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestRouter(t *testing.T) (http.Handler, *Runtime) {
	t.Helper()
	runtime, err := NewRuntime(context.Background(), "")
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	return Router(runtime, "../../web"), runtime
}

func doJSON(t *testing.T, router http.Handler, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	var decoded map[string]any
	if recorder.Body.Len() > 0 {
		if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode %s body %q: %v", path, recorder.Body.String(), err)
		}
	}
	return recorder.Code, decoded
}

func TestPlanCreateReturns422WithStowageDetails(t *testing.T) {
	router, _ := newTestRouter(t)
	if status, body := doJSON(t, router, http.MethodPut, "/api/stowage/config",
		`{"chamber_id":"chamber-1","effective_length_m":100,"safety_gap_m":10}`); status != http.StatusOK {
		t.Fatalf("config status=%d body=%v", status, body)
	}
	// 60 + 10 + 60 = 130 > 100, so no plan may be created.
	status, body := doJSON(t, router, http.MethodPost, "/api/plans", `{
		"chamber_id":"chamber-1","direction":"upstream",
		"vessels":[
			{"name":"Alpha","call_sign":"A-1","length_m":60},
			{"name":"Bravo","call_sign":"B-2","length_m":60}
		]}`)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%v", status, body)
	}
	if body["code"] != "stowage_exceeded" {
		t.Fatalf("expected stowage_exceeded code, got %v", body["code"])
	}
	report, ok := body["stowage"].(map[string]any)
	if !ok {
		t.Fatalf("expected stowage report in body: %v", body)
	}
	if report["vessel_count"].(float64) != 1 {
		t.Fatalf("expected one loaded vessel, got %v", report["vessel_count"])
	}
	deferred := report["deferred"].([]any)
	first := deferred[0].(map[string]any)
	if first["name"] != "Bravo" || first["queue_position"].(float64) != 2 {
		t.Fatalf("unexpected deferred vessel: %v", first)
	}
	if status, plans := doJSON(t, router, http.MethodGet, "/api/plans", ""); status != http.StatusOK {
		t.Fatalf("list plans: %d", status)
	} else if listed, _ := plans["chamber-1"].([]any); len(listed) != 0 {
		t.Fatalf("no plan may exist after rejection, got %d", len(listed))
	}
}

func TestStowagePreviewReportsOverflowWithoutPersisting(t *testing.T) {
	router, _ := newTestRouter(t)
	status, body := doJSON(t, router, http.MethodPost, "/api/stowage/preview", `{
		"chamber_id":"chamber-1",
		"vessels":[{"name":"Giant","call_sign":"G-9","length_m":9999}]}`)
	if status != http.StatusOK {
		t.Fatalf("expected preview 200, got %d body=%v", status, body)
	}
	if body["exceeded"] != true || body["vessel_count"].(float64) != 0 {
		t.Fatalf("unexpected preview: %v", body)
	}
	if _, plans := doJSON(t, router, http.MethodGet, "/api/plans", ""); plans != nil {
		if listed, _ := plans["chamber-1"].([]any); len(listed) != 0 {
			t.Fatal("preview must not persist a plan")
		}
	}
}

func TestStowageConfigValidation(t *testing.T) {
	router, _ := newTestRouter(t)
	if status, _ := doJSON(t, router, http.MethodPut, "/api/stowage/config",
		`{"chamber_id":"chamber-1","effective_length_m":0,"safety_gap_m":5}`); status != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-positive length")
	}
	status, body := doJSON(t, router, http.MethodGet, "/api/stowage/config", "")
	if status != http.StatusOK {
		t.Fatalf("config list status=%d", status)
	}
	chamber := body["chamber-1"].(map[string]any)
	if chamber["effective_length_m"].(float64) != 280 {
		t.Fatalf("expected default effective length 280, got %v", chamber["effective_length_m"])
	}
}
