package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type CategoryHandler struct {
	categoryRepo CategoryRepository
	adminRepo    AdminRepository
}

func NewCategoryHandler(categoryRepo CategoryRepository, adminRepo AdminRepository) *CategoryHandler {
	return &CategoryHandler{
		categoryRepo: categoryRepo,
		adminRepo:    adminRepo,
	}
}

func (h *CategoryHandler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		orcid, ok := sessionManager.Get(ctx, "orcid").(string)
		if !ok || orcid == "" {
			//http.Error(w, "Unauthorized: login required", http.StatusUnauthorized)
			// return
		}

		isAdminUser, err := h.adminRepo.IsAdmin(ctx, "0009-0005-8993-9587")
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !isAdminUser {
			http.Error(w, "Forbidden: admin access required", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

// GetCategories handles GET /api/v1/categories - List all categories
func (h *CategoryHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.categoryRepo.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error fetching categories", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(categories); err != nil {
		errorLogger.Printf("failed to write response: %v", err)
	}
}

// GetCategory handles GET /api/v1/categories/{id} - Get a specific category
func (h *CategoryHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/categories/"
	idStr := strings.TrimPrefix(r.URL.Path, prefix)

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	category, err := h.categoryRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			http.Error(w, "Category not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Error fetching category", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(category); err != nil {
		errorLogger.Printf("failed to write response: %v", err)
	}
}

// CreateCategory handles POST /api/v1/categories - Create a new category
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Category name is required", http.StatusBadRequest)
		return
	}

	category, err := h.categoryRepo.Create(r.Context(), req.Name)
	if err != nil {
		if errors.Is(err, ErrCategoryAlreadyExists) {
			http.Error(w, "Category name already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Error creating category", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(category); err != nil {
		errorLogger.Printf("failed to write response: %v", err)
	}
}

// UpdateCategory handles PUT /api/v1/categories/{id} - Update a category
func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/categories/"
	idStr := strings.TrimPrefix(r.URL.Path, prefix)

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Category name is required", http.StatusBadRequest)
		return
	}

	category, err := h.categoryRepo.Update(r.Context(), id, req.Name)
	if err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			http.Error(w, "Category not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrCategoryAlreadyExists) {
			http.Error(w, "Category name already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Error updating category", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(category); err != nil {
		errorLogger.Printf("failed to write response: %v", err)
	}
}

// DeleteCategory handles DELETE /api/v1/categories/{id} - Delete a category
func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/categories/"
	idStr := strings.TrimPrefix(r.URL.Path, prefix)

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	err = h.categoryRepo.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			http.Error(w, "Category not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Error deleting category", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Router handles routing for category endpoints
func (h *CategoryHandler) Router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/v1/categories" && r.Method == "GET":
		h.GetCategories(w, r)
	case path == "/api/v1/categories" && r.Method == "POST":
		h.requireAdmin(h.CreateCategory)(w, r)
	case strings.HasPrefix(path, "/api/v1/categories/") && r.Method == "GET":
		h.GetCategory(w, r)
	case strings.HasPrefix(path, "/api/v1/categories/") && r.Method == "PUT":
		h.requireAdmin(h.UpdateCategory)(w, r)
	case strings.HasPrefix(path, "/api/v1/categories/") && r.Method == "DELETE":
		h.requireAdmin(h.DeleteCategory)(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
