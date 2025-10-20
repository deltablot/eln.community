package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// MockCategoryRepository implements CategoryRepository for testing
type MockCategoryRepository struct {
	categories map[int64]*Category
	nextID     int64
}

func (m *MockCategoryRepository) AssociateCategoryWithRecord(ctx context.Context, tx *sql.Tx, recordID string, categoryID int64) error {
	return nil
}

func (m *MockCategoryRepository) GetRecordCategories(ctx context.Context, recordID string) ([]Category, error) {
	return []Category{}, nil
}

func NewMockCategoryRepository() *MockCategoryRepository {
	return &MockCategoryRepository{
		categories: make(map[int64]*Category),
		nextID:     1,
	}
}

func (m *MockCategoryRepository) GetAll(ctx context.Context) ([]Category, error) {
	var categories []Category
	for _, cat := range m.categories {
		categories = append(categories, *cat)
	}
	return categories, nil
}

func (m *MockCategoryRepository) GetByID(ctx context.Context, id int64) (*Category, error) {
	if cat, exists := m.categories[id]; exists {
		return cat, nil
	}
	return nil, ErrCategoryNotFound
}

func (m *MockCategoryRepository) Create(ctx context.Context, name string) (*Category, error) {
	// Check for duplicate names
	for _, cat := range m.categories {
		if cat.Name == name {
			return nil, ErrCategoryAlreadyExists
		}
	}

	category := &Category{
		Id:         m.nextID,
		Name:       name,
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}
	m.categories[m.nextID] = category
	m.nextID++
	return category, nil
}

func (m *MockCategoryRepository) Update(ctx context.Context, id int64, name string) (*Category, error) {
	if _, exists := m.categories[id]; !exists {
		return nil, ErrCategoryNotFound
	}

	// Check for duplicate names (excluding current category)
	for catID, cat := range m.categories {
		if catID != id && cat.Name == name {
			return nil, ErrCategoryAlreadyExists
		}
	}

	category := m.categories[id]
	category.Name = name
	category.ModifiedAt = time.Now()
	return category, nil
}

func (m *MockCategoryRepository) Delete(ctx context.Context, id int64) error {
	if _, exists := m.categories[id]; !exists {
		return ErrCategoryNotFound
	}
	delete(m.categories, id)
	return nil
}

// MockAdminRepository implements AdminRepository for testing
type MockAdminRepository struct {
	adminOrcids map[string]bool
}

func NewMockAdminRepository() *MockAdminRepository {
	return &MockAdminRepository{
		adminOrcids: make(map[string]bool),
	}
}

func (m *MockAdminRepository) IsAdmin(ctx context.Context, orcid string) (bool, error) {
	return m.adminOrcids[orcid], nil
}

func (m *MockAdminRepository) AddAdmin(orcid string) {
	m.adminOrcids[orcid] = true
}

func TestCategoryHandler_GetCategories(t *testing.T) {
	// Setup
	mockCategoryRepo := NewMockCategoryRepository()
	mockAdminRepo := NewMockAdminRepository()
	handler := NewCategoryHandler(mockCategoryRepo, mockAdminRepo)

	// Add test data
	mockCategoryRepo.Create(context.Background(), "Chemistry")
	mockCategoryRepo.Create(context.Background(), "Physics")

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/categories", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.GetCategories(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var categories []Category
	if err := json.NewDecoder(w.Body).Decode(&categories); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(categories))
	}
}

func TestCategoryHandler_GetCategory(t *testing.T) {
	// Setup
	mockCategoryRepo := NewMockCategoryRepository()
	mockAdminRepo := NewMockAdminRepository()
	handler := NewCategoryHandler(mockCategoryRepo, mockAdminRepo)

	// Add test data
	_, _ = mockCategoryRepo.Create(context.Background(), "Chemistry")

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/categories/1", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.GetCategory(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var result Category
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Name != "Chemistry" {
		t.Errorf("Expected category name 'Chemistry', got '%s'", result.Name)
	}
}

func TestCategoryHandler_GetCategory_NotFound(t *testing.T) {
	// Setup
	mockCategoryRepo := NewMockCategoryRepository()
	mockAdminRepo := NewMockAdminRepository()
	handler := NewCategoryHandler(mockCategoryRepo, mockAdminRepo)

	// Create request for non-existent category
	req := httptest.NewRequest("GET", "/api/v1/categories/999", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.GetCategory(w, req)

	// Assert
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestCategoryHandler_CreateCategory(t *testing.T) {
	// Setup
	mockCategoryRepo := NewMockCategoryRepository()
	mockAdminRepo := NewMockAdminRepository()
	handler := NewCategoryHandler(mockCategoryRepo, mockAdminRepo)

	// Create request body
	reqBody := map[string]string{"name": "Biology"}
	jsonBody, _ := json.Marshal(reqBody)

	// Create request
	req := httptest.NewRequest("POST", "/api/v1/categories", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.CreateCategory(w, req)

	// Assert
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var result Category
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Name != "Biology" {
		t.Errorf("Expected category name 'Biology', got '%s'", result.Name)
	}
}

func TestCategoryHandler_CreateCategory_DuplicateName(t *testing.T) {
	// Setup
	mockCategoryRepo := NewMockCategoryRepository()
	mockAdminRepo := NewMockAdminRepository()
	handler := NewCategoryHandler(mockCategoryRepo, mockAdminRepo)

	// Add existing category
	mockCategoryRepo.Create(context.Background(), "Chemistry")

	// Create request body with duplicate name
	reqBody := map[string]string{"name": "Chemistry"}
	jsonBody, _ := json.Marshal(reqBody)

	// Create request
	req := httptest.NewRequest("POST", "/api/v1/categories", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.CreateCategory(w, req)

	// Assert
	if w.Code != http.StatusConflict {
		t.Errorf("Expected status %d, got %d", http.StatusConflict, w.Code)
	}
}
