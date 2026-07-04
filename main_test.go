package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveStreamIDUsesQueryParamFirst(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/view/jumail?stream=other", nil)
	got := resolveStreamID(req)
	if got != "other" {
		t.Fatalf("expected stream from query param, got %q", got)
	}
}

func TestResolveStreamIDFallsBackToPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/view/jumail", nil)
	got := resolveStreamID(req)
	if got != "jumail" {
		t.Fatalf("expected stream from path, got %q", got)
	}
}
