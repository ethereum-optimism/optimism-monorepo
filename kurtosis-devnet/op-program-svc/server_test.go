package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func createTestServer(t *testing.T) (*server, *MockProofFS, *MockBuilder) {
	t.Helper()
	mockProofFS := NewMockProofFS()
	mockBuilder := &MockBuilder{}

	srv := &server{
		appRoot:    "test-root",
		configsDir: "test-configs",
		buildDir:   "test-build",
		buildCmd:   "test-cmd",
		port:       8080,
		proofFS:    mockProofFS,
		builder:    mockBuilder,
	}

	return srv, mockProofFS, mockBuilder
}

func createMultipartRequest(t *testing.T, files map[string][]byte) (*http.Request, error) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for filename, content := range files {
		part, err := writer.CreateFormFile("files[]", filename)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestHandleUpload_MethodNotAllowed(t *testing.T) {
	srv, _, _ := createTestServer(t)

	req := httptest.NewRequest("GET", "/upload", nil)
	w := httptest.NewRecorder()

	srv.handleUpload(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleUpload_NoFiles(t *testing.T) {
	srv, _, _ := createTestServer(t)

	req := httptest.NewRequest("POST", "/upload", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	w := httptest.NewRecorder()

	srv.handleUpload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleUpload_Success(t *testing.T) {
	srv, mockProofFS, mockBuilder := createTestServer(t)

	// Setup test data
	files := map[string][]byte{
		"test.txt": []byte("test content"),
	}

	// Setup mocks
	mockBuilder.saveUploadedFilesFn = func(files []UploadedFile) error {
		return nil
	}
	mockBuilder.executeBuildFn = func() ([]byte, error) {
		return []byte("build successful"), nil
	}
	mockProofFS.scanProofFilesFn = func() error {
		return nil
	}

	// Create request
	req, err := createMultipartRequest(t, files)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	srv.handleUpload(w, req)

	// We now expect 200 OK instead of a redirect
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHandleUpload_SaveError(t *testing.T) {
	srv, _, mockBuilder := createTestServer(t)

	// Setup test data
	files := map[string][]byte{
		"test.txt": []byte("test content"),
	}

	// Setup mock to return error
	mockBuilder.saveUploadedFilesFn = func(files []UploadedFile) error {
		return fmt.Errorf("failed to save files")
	}

	// Create request
	req, err := createMultipartRequest(t, files)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	srv.handleUpload(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestHandleUpload_BuildError(t *testing.T) {
	srv, _, mockBuilder := createTestServer(t)

	// Setup test data
	files := map[string][]byte{
		"test.txt": []byte("test content"),
	}

	// Setup mocks
	mockBuilder.saveUploadedFilesFn = func(files []UploadedFile) error {
		return nil
	}
	mockBuilder.executeBuildFn = func() ([]byte, error) {
		return []byte("build failed"), fmt.Errorf("build error")
	}

	// Create request
	req, err := createMultipartRequest(t, files)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	srv.handleUpload(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, w.Code)
	}

	if !strings.Contains(w.Body.String(), "build error") {
		t.Errorf("Expected error message to contain 'build error', got %s", w.Body.String())
	}
}

func TestHandleUpload_ScanError(t *testing.T) {
	srv, mockProofFS, mockBuilder := createTestServer(t)

	// Setup test data
	files := map[string][]byte{
		"test.txt": []byte("test content"),
	}

	// Setup mocks
	mockBuilder.saveUploadedFilesFn = func(files []UploadedFile) error {
		return nil
	}
	mockBuilder.executeBuildFn = func() ([]byte, error) {
		return []byte("build successful"), nil
	}
	mockProofFS.scanProofFilesFn = func() error {
		return fmt.Errorf("scan error")
	}

	// Create request
	req, err := createMultipartRequest(t, files)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	srv.handleUpload(w, req)

	// Even with scan error, we should still return 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHandleUpload_UnchangedFiles(t *testing.T) {
	srv, _, _ := createTestServer(t)

	// Setup test data
	files := map[string][]byte{
		"test.txt": []byte("test content"),
	}

	// First request
	req1, err := createMultipartRequest(t, files)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w1 := httptest.NewRecorder()
	srv.handleUpload(w1, req1)

	// First request should return 200 OK
	if w1.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w1.Code)
	}

	// Second request with same files
	req2, err := createMultipartRequest(t, files)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	w2 := httptest.NewRecorder()
	srv.handleUpload(w2, req2)

	// Second request with unchanged files should return 304 Not Modified
	if w2.Code != http.StatusNotModified {
		t.Errorf("Expected status code %d, got %d", http.StatusNotModified, w2.Code)
	}

	if !strings.Contains(w2.Body.String(), "Files unchanged") {
		t.Errorf("Expected response to contain 'Files unchanged', got %s", w2.Body.String())
	}
}
