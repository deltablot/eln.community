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
		// Get organization name from cache or API
		var orgName string
		if h.rorNameCache != nil {
			if name, found := h.rorNameCache.Get(rorId); found {
				orgName = name
			}
		}

		// Fallback to API if not in cache
		if orgName == "" && h.rorClient != nil {
			if org, err := h.rorClient.GetOrganization(rorId); err == nil {
				orgName = org.Name
			} else {
				log.Printf("Error fetching organization name for %s: %v", rorId, err)
				orgName = rorId // Fallback to ID
			}
		}

		// Count records for this organization
		_, count, err := h.recordRepo.GetAllByRorIDsPaginated(ctx, []string{rorId}, 1, 0)
		if err != nil {
			log.Printf("Error counting records for ROR %s: %v", rorId, err)
			continue
		}

		// Only include organizations with at least one record
		if count > 0 {
			organizations = append(organizations, OrganizationInfo{
				ID:          rorId,
				Name:        orgName,
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
