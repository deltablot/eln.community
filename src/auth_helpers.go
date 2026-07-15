package main

import (
	"net/http"
    "context"
)

const (
    sessionKeyOrcid = "orcid"
    sessionKeyName = "name"
)

type adminChecker interface {
    IsAdmin(ctx context.Context, orcid string) (bool, error)
}

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

func requireAuthenticatedUser(w http.ResponseWriter, r *http.Request, source string) (*User, bool) {
    user, ok := userFromSession(r.Context())
    if !ok {
        errorLogger.Printf("%s: authentication required: method %q, path %q", source, r.Method, r.URL.Path)
        http.Error(w, "authentication required", http.StatusUnauthorized)
        return nil, false
    }
    return user, true
}

func currentUserIsAdmin(w http.ResponseWriter, r *http.Request, source string, adminRepo adminChecker) (bool, bool) {
    user, ok := userFromSession(r.Context())
    if !ok {
        return false, true
    }

    isAdmin, err := adminRepo.IsAdmin(r.Context(), user.Orcid)
    if err != nil {
        errorLogger.Printf("%s: failed to check admin status for orcid %q: %v", source, user.Orcid, err)
		http.Error(w, "failed to check admin permissions", http.StatusInternalServerError)
		return false, false
    }
    return isAdmin, true
}

func requireAdminUser(w http.ResponseWriter, r *http.Request, source string, adminRepo adminChecker) (*User, bool) {
	user, ok := requireAuthenticatedUser(w, r, source)
	if !ok {
		return nil, false
	}

	isAdmin, err := adminRepo.IsAdmin(r.Context(), user.Orcid)
	if err != nil {
		errorLogger.Printf("%s: failed to check admin status for orcid %q: %v", source, user.Orcid, err)
		http.Error(w, "failed to check admin permissions", http.StatusInternalServerError)
		return nil, false
	}

	if !isAdmin {
		errorLogger.Printf("%s: admin access required for orcid %q", source, user.Orcid)
		http.Error(w, "admin access required", http.StatusForbidden)
		return nil, false
	}

	return user, true
}
