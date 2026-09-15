package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

const categoryHandlerErr = "Error: category handler:"

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
		_, err := requireAdminUser(w, r, h.adminRepo)
		if err != nil {
			return
		}
		next(w, r)
	}
}

// GetCategories handles GET /api/v1/categories - List all categories
func (h *CategoryHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	res := APIResponse[Category]{}
	// Check if hierarchical view is requested
	hierarchical := r.URL.Query().Get("hierarchical") == "true"

	var categories []Category
	var err error

	if hierarchical {
		categories, err = h.categoryRepo.GetAllHierarchical(r.Context())
	} else {
		categories, err = h.categoryRepo.GetAll(r.Context())
	}

	if err != nil {
		res.Data = []Category{}
		status := http.StatusInternalServerError
		res.Meta.Error.Code = status
		res.Meta.Error.Message = http.StatusText(status)
		res.Meta.Error.Description = "database error"
		errorLogger.Printf("%s: failed to get categories: %v", categoryHandlerErr, err)
		writeJson(w, status, res)
		return
	}

	res.Data = categories
	writeJson(w, http.StatusOK, res)
}

// GetCategory handles GET /api/v1/categories/{id} - Get a specific category
func (h *CategoryHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	res := APIResponse[Category]{}
	const prefix = "/api/v1/categories/"
	idStr := strings.TrimPrefix(r.URL.Path, prefix)

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		res.Data = []Category{}
		status := http.StatusBadRequest
		res.Meta.Error = ResponseError{
			Code:        status,
			Message:     http.StatusText(status),
			Description: "invalid category ID",
		}
		errorLogger.Printf("%s syntax error: invalid category %d: %v", categoryHandlerErr, id, err)
		writeJson(w, status, res)
		return
	}

	category, err := h.categoryRepo.GetByID(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		description := "database error"
		if errors.Is(err, ErrCategoryNotFound) {
			status = http.StatusNotFound
			description = "category not found"
		} else {
			errorLogger.Printf("%s: failed to fetch category %d: %v", categoryHandlerErr, id, err)
		}
		res.Data = []Category{}
		res.Meta.Error = ResponseError{
			Code:        status,
			Message:     http.StatusText(status),
			Description: description,
		}
		writeJson(w, status, res)
		return
	}

	res.Data = []Category{*category}
	writeJson(w, http.StatusOK, res)
}

// CreateCategory handles POST /api/v1/categories - Create a new category
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		ParentId *int64 `json:"parent_id,omitempty"`
	}

	if err := requireJSONBody(w, r, &req); err != nil {
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Category name is required", http.StatusBadRequest)
		return
	}

	category, err := h.categoryRepo.Create(r.Context(), req.Name, req.ParentId)
	if err != nil {
		if errors.Is(err, ErrCategoryAlreadyExists) {
			http.Error(w, "Category name already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Error creating category", http.StatusInternalServerError)
		return
	}

	writeJson(w, http.StatusCreated, category)
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
		Name     string `json:"name"`
		ParentId *int64 `json:"parent_id,omitempty"`
	}
	if err := requireJSONBody(w, r, &req); err != nil {
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Category name is required", http.StatusBadRequest)
		return
	}

	category, err := h.categoryRepo.Update(r.Context(), id, req.Name, req.ParentId)
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
