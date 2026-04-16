/**
 * eln.community
 * © 2025 - Nicolas CARPi, Deltablot
 * License: AGPLv3
 */
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
    "github.com/alexedwards/scs/v2"
	"golang.org/x/oauth2"
)

var (
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	oidcVerifier *oidc.IDTokenVerifier
)

func InitOIDC(siteUrl string) {
	ctx := context.Background()
	var err error

	// discover provider
	provider, err = oidc.NewProvider(ctx, "https://orcid.org")
	if err != nil {
		log.Fatalf("failed to discover ORCID: %v", err)
	}

	// setup clientID
	clientID := os.Getenv("ORCID_CLIENT_ID")
	oidcVerifier = provider.Verifier(&oidc.Config{ClientID: clientID})

	clientSecret := os.Getenv("ORCID_CLIENT_SECRET")
	redirectURL := siteUrl + "/auth/callback"
	// Configure oAuth2
	oauth2Config = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  redirectURL,
		Scopes:       []string{oidc.ScopeOpenID},
	}
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// 1) Redirect user to ORCID
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	// Store state in a cookie (or session) to verify later:
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(5 * time.Minute),
	})
	url := oauth2Config.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}

// 2) Handle ORCID callback
func CallbackHandler(sm *scs.SessionManager, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify state
	cookie, err := r.Cookie("oidc_state")
	if err != nil || r.URL.Query().Get("state") != cookie.Value {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := oauth2Config.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "Token exchange failed", http.StatusInternalServerError)
		return
	}

	// Extract & verify the ID Token
	rawID := token.Extra("id_token").(string)
	idToken, err := oidcVerifier.Verify(ctx, rawID)
	if err != nil {
		http.Error(w, "ID token verification failed", http.StatusInternalServerError)
		return
	}

	// Pull claims into a user struct
	var claims struct {
		Sub        string `json:"sub"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "Failed to decode claims", http.StatusInternalServerError)
		return
	}

	// now fetch UserInfo (where ORCID actually returns name/email)
	userInfo, err := provider.UserInfo(ctx, oauth2Config.TokenSource(ctx, token))
	if err != nil {
		http.Error(w, "Failed to get userinfo", http.StatusInternalServerError)
		return
	}

	// decode the standard claims
	var profile struct {
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
	}
	if err := userInfo.Claims(&profile); err != nil {
		http.Error(w, "Failed to parse userinfo", http.StatusInternalServerError)
		return
	}

	// store orcid and name in session
	//sessionManager.Put(ctx, "orcid", claims.Sub)
	//sessionManager.Put(ctx, "name", strings.TrimSpace(profile.GivenName+" "+profile.FamilyName))
	sm.Put(ctx, "orcid", claims.Sub)
	sm.Put(ctx, "name", strings.TrimSpace(profile.GivenName+" "+profile.FamilyName))

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
