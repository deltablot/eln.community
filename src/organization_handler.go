package main

import (
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"
)

type OrganizationHandler struct {
	rorRepo      RorRepository
	rorNameCache *RorNameCache
	rorClient    *RorClient
	recordRepo   RecordRepository
}

type OrganizationInfo struct {
	ID          string
	Name        string
	CountryName string
	RecordCount int
}

func NewOrganizationHandler(rorRepo RorRepository, rorNameCache *RorNameCache, rorClient *RorClient, recordRepo RecordRepository) *OrganizationHandler {
	return &OrganizationHandler{
		rorRepo:      rorRepo,
		rorNameCache: rorNameCache,
		rorClient:    rorClient,
		recordRepo:   recordRepo,
	}
}

// GetOrganizationsPage displays all organizations that have uploaded templates
func (h *OrganizationHandler) GetOrganizationsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all unique ROR IDs from the database
	rorIds, err := h.rorRepo.GetAllUniqueRorIds(ctx)
	if err != nil {
		log.Printf("Error fetching ROR IDs: %v", err)
		http.Error(w, "Error fetching organizations", http.StatusInternalServerError)
		return
	}

	// Get organization details and count records for each
	organizations := make([]OrganizationInfo, 0, len(rorIds))
	for _, rorId := range rorIds {
		// Get organization details from API (includes country)
		var orgName string
		var countryName string

		if h.rorClient != nil {
			if org, err := h.rorClient.GetOrganization(rorId); err == nil {
				orgName = org.Name
				if org.Country != nil {
					countryName = org.Country.CountryName
				}
			} else {
				log.Printf("Error fetching organization for %s: %v", rorId, err)
				// Fallback to name cache
				if h.rorNameCache != nil {
					if name, found := h.rorNameCache.Get(rorId); found {
						orgName = name
					}
				}
				if orgName == "" {
					orgName = rorId // Final fallback to ID
				}
			}
		} else if h.rorNameCache != nil {
			// No ROR client, use name cache only
			if name, found := h.rorNameCache.Get(rorId); found {
				orgName = name
			} else {
				orgName = rorId
			}
		} else {
			orgName = rorId
		}

		// Count records for this organization
		_, count, err := h.recordRepo.GetAllByRorIDsPaginated(ctx, []string{rorId}, 1, 0, "created_at", "desc", make(map[string]interface{}))
		if err != nil {
			log.Printf("Error counting records for ROR %s: %v", rorId, err)
			continue
		}

		// Only include organizations with at least one record
		if count > 0 {
			organizations = append(organizations, OrganizationInfo{
				ID:          rorId,
				Name:        orgName,
				CountryName: countryName,
				RecordCount: count,
			})
		}
	}

	// Sort organizations by name
	sort.Slice(organizations, func(i, j int) bool {
		return strings.ToLower(organizations[i].Name) < strings.ToLower(organizations[j].Name)
	})

	// Get current user info
	var user *User
	if orcid, ok := sessionManager.Get(ctx, "orcid").(string); ok {
		name, _ := sessionManager.Get(ctx, "name").(string)
		user = &User{
			Name:  name,
			Orcid: orcid,
		}
	}

	// Render template
	var pageTmpl = template.Must(template.ParseFS(staticFiles,
		"templates/layout.html",
		"templates/organizations.html",
	))

	data := struct {
		App           App
		Organizations []OrganizationInfo
		User          *User
		CurrentPage   string
	}{
		App:           app,
		Organizations: organizations,
		User:          user,
		CurrentPage:   "organizations",
	}

	w.Header().Set("Content-Type", "text/html")
	if err := pageTmpl.ExecuteTemplate(w, "layout", data); err != nil {
		errorLogger.Printf("template exec error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}
