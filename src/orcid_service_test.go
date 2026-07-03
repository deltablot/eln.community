package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetEmailReturnsUnavailableWhenNoPublicEmail(t *testing.T) {
	ctx := context.Background()
	orcid := "0000-0000-0000-0000"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/" + orcid + "/email"
		if r.URL.Path != expectedPath {
			t.Fatalf("expected path: %q, got: %q", expectedPath, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"email":[]}`))
	}))
	defer server.Close()

	orcidClient := &OrcidClient{
		Url: server.URL + "/",
	}
	email, err := orcidClient.GetEmail(ctx, orcid)

	if email != "" {
		t.Fatalf("expected empty email, got %q", email)
	}

	var emailUnavailable *EmailUnavailable
	if !errors.As(err, &emailUnavailable) {
		t.Fatalf("expected EmailUnavailable error, got %T: %v", err, err)
	}

	if emailUnavailable.Orcid != orcid {
		t.Fatalf("expected orcid %q, got %q", orcid, emailUnavailable.Orcid)
	}
}

func TestGetEmailReturnsEmail(t *testing.T) {
	ctx := context.Background()
	orcid := "0000-0000-0000-0000"
	expectedEmail := "test@test.com"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/" + orcid + "/email"
		if r.URL.Path != expectedPath {
			t.Fatalf("expected path: %q, got: %q", expectedPath, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"email":[ {"email":"test@test.com"} ]}`))
	}))
	defer server.Close()

	orcidClient := &OrcidClient{
		Url: server.URL + "/",
	}
	email, err := orcidClient.GetEmail(ctx, orcid)
	if err != nil {
		t.Fatalf("expected GetEmail to succeed, got error: %v", err)
	}

	if email != expectedEmail {
		t.Fatalf("expected: %q, got: %q", expectedEmail, email)
	}
}
