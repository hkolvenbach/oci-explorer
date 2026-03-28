package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBadgeSVGMissingImage(t *testing.T) {
	req := httptest.NewRequest("GET", "/badge/score.svg", nil)
	w := httptest.NewRecorder()

	handleBadgeSVG(w, req)

	resp := w.Result()
	if resp.Header.Get("Content-Type") != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", resp.Header.Get("Content-Type"))
	}
	body := w.Body.String()
	if !strings.Contains(body, "<svg") {
		t.Error("response is not SVG")
	}
	if !strings.Contains(body, "error") {
		t.Error("error badge should contain 'error'")
	}
}

func TestBadgeJSONMissingImage(t *testing.T) {
	req := httptest.NewRequest("GET", "/badge/score.json", nil)
	w := httptest.NewRecorder()

	handleBadgeJSON(w, req)

	resp := w.Result()
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", resp.Header.Get("Content-Type"))
	}
	body := w.Body.String()
	if !strings.Contains(body, `"isError"`) {
		t.Error("error JSON should contain isError field")
	}
	if !strings.Contains(body, `"schemaVersion"`) {
		t.Error("JSON should contain schemaVersion")
	}
}

func TestBadgeSVGCacheControl(t *testing.T) {
	req := httptest.NewRequest("GET", "/badge/score.svg", nil)
	w := httptest.NewRecorder()

	handleBadgeSVG(w, req)

	cc := w.Result().Header.Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("error badge Cache-Control = %q, want no-cache", cc)
	}
}
