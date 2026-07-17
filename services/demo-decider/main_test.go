package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecideRequiresAllowedOrigin(t *testing.T) {
	application := &app{allowedOrigins: parseOrigins("https://111.229.205.99")}
	request := httptest.NewRequest(http.MethodPost, "/api/decide", bytes.NewBufferString(`{"options":["a","b"]}`))
	response := httptest.NewRecorder()
	application.routes().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestDecideReturnsSubmittedChoice(t *testing.T) {
	application := &app{allowedOrigins: parseOrigins("https://111.229.205.99")}
	request := httptest.NewRequest(http.MethodPost, "/api/decide", bytes.NewBufferString(`{"options":["tea","coffee"]}`))
	request.Header.Set("Origin", "https://111.229.205.99")
	response := httptest.NewRecorder()
	application.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	var output decideResponse
	if err := json.Unmarshal(response.Body.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Choice != "tea" && output.Choice != "coffee" {
		t.Fatalf("unexpected choice %q", output.Choice)
	}
}

func TestNormalizeOptions(t *testing.T) {
	options, err := normalizeOptions([]string{" first ", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if options[0] != "first" {
		t.Fatalf("first option = %q", options[0])
	}
	if _, err := normalizeOptions([]string{"only one"}); err == nil {
		t.Fatal("expected validation error")
	}
}
