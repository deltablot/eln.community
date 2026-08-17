package main

import (
	"context"
	"errors"
	"net/http"
)

const (
	sessionKeyOrcid = "orcid"
	sessionKeyName  = "name"
)

type adminChecker interface {
	IsAdmin(ctx context.Context, orcid string) (bool, error)
}

var (
	ErrAuthRequired     = errors.New("authentication required")
	ErrAdminPermissions = errors.New("failed to check admin permissions")
	ErrAdminRequired    = errors.New("admin access required")
)

// https://pkg.go.dev/github.com/alexedwards/scs/v2#SessionManager
func userFromSession(ctx context.Context) (*User, bool) {
	orcid, ok := sessionManager.Get(ctx, sessionKeyOrcid).(string)
	if !ok || orcid == "" {
		return nil, false
	}

	name, _ := sessionManager.Get(ctx, sessionKeyName).(string)
	return &User{
		Name:  name,
		Orcid: orcid,
	}, true
}

func requireAuthenticatedUser(w http.ResponseWriter, r *http.Request) (*User, error) {
	user, ok := userFromSession(r.Context())
	if !ok {
		errorLogger.Printf("%s: method %q, path %q", ErrAuthRequired, r.Method, r.URL.Path)
		http.Error(w, ErrAuthRequired.Error(), http.StatusUnauthorized)
		return nil, ErrAuthRequired
	}
	return user, nil
}

func currentUserIsAdmin(w http.ResponseWriter, r *http.Request, adminRepo adminChecker) (bool, error) {
	user, ok := userFromSession(r.Context())
	if !ok {
		return false, nil
	}

	isAdmin, err := adminRepo.IsAdmin(r.Context(), user.Orcid)
	if err != nil {
		errorLogger.Printf("%s for orcid %q: %v", ErrAdminPermissions, user.Orcid, err)
		http.Error(w, ErrAdminPermissions.Error(), http.StatusInternalServerError)
		return false, ErrAdminPermissions
	}
	return isAdmin, nil
}

func requireAdminUser(w http.ResponseWriter, r *http.Request, adminRepo adminChecker) (*User, error) {
	user, err := requireAuthenticatedUser(w, r)
	if err != nil {
		return nil, err
	}

	isAdmin, err := adminRepo.IsAdmin(r.Context(), user.Orcid)
	if err != nil {
		errorLogger.Printf("%s for orcid %q: %v", ErrAdminPermissions, user.Orcid, err)
		http.Error(w, ErrAdminPermissions.Error(), http.StatusInternalServerError)
		return nil, ErrAdminPermissions
	}

	if !isAdmin {
		errorLogger.Printf("%s for orcid %q", ErrAdminRequired, user.Orcid)
		http.Error(w, ErrAdminRequired.Error(), http.StatusForbidden)
		return nil, ErrAdminRequired
	}

	return user, nil
}
