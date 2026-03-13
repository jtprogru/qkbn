package qkbnhttp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fileExists проверяет существование файла.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHome bool
	}{
		{
			name:     "tilde expansion",
			input:    "~/.qwen/todos",
			wantHome: true,
		},
		{
			name:     "tilde with slash",
			input:    "~/test",
			wantHome: true,
		},
		{
			name:     "absolute path",
			input:    "/tmp/test",
			wantHome: false,
		},
		{
			name:     "relative path",
			input:    "test/path",
			wantHome: false,
		},
		{
			name:     "tilde only",
			input:    "~",
			wantHome: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)

			if tt.wantHome {
				home, _ := os.UserHomeDir()
				if !strings.HasPrefix(result, home) {
					t.Errorf("expandPath(%q) = %q, want prefix %q", tt.input, result, home)
				}
			}
		})
	}
}

func TestGetLatestFile(t *testing.T) {
	tmpDir := t.TempDir()

	files := []struct {
		name    string
		content string
		delay   time.Duration
	}{
		{"old.json", `{"todos": []}`, 0},
		{"middle.json", `{"todos": []}`, 100 * time.Millisecond},
		{"latest.json", `{"todos": []}`, 200 * time.Millisecond},
	}

	for i, f := range files {
		path := filepath.Join(tmpDir, f.name)

		err := os.WriteFile(path, []byte(f.content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", f.name, err)
		}

		modTime := time.Now().Add(-time.Duration(len(files)-i-1) * 100 * time.Millisecond)
		os.Chtimes(path, modTime, modTime)
		time.Sleep(f.delay)
	}

	t.Run("returns latest modified file", func(t *testing.T) {
		result, err := getLatestFile(tmpDir)
		if err != nil {
			t.Fatalf("getLatestFile() error = %v", err)
		}

		if !strings.HasSuffix(result, "latest.json") {
			t.Errorf("getLatestFile() = %q, want suffix latest.json", result)
		}
	})

	t.Run("empty directory returns error", func(t *testing.T) {
		emptyDir := t.TempDir()

		_, emptyErr := getLatestFile(emptyDir)
		if emptyErr == nil {
			t.Error("getLatestFile() expected error for empty directory")
		}
	})

	t.Run("non-existent directory returns error", func(t *testing.T) {
		_, err := getLatestFile("/non/existent/path")
		if err == nil {
			t.Error("getLatestFile() expected error for non-existent directory")
		}
	})
}

func TestGetLatestFile_IgnoresNonJSON(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "data.json"), []byte(`{"todos": []}`), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Readme"), 0644)

	result, err := getLatestFile(tmpDir)
	if err != nil {
		t.Fatalf("getLatestFile() error = %v", err)
	}

	if !strings.HasSuffix(result, "data.json") {
		t.Errorf("getLatestFile() = %q, want data.json", result)
	}
}

func TestGetAllJSONFiles(t *testing.T) {
	tmpDir := t.TempDir()

	files := []struct {
		name    string
		content string
		delay   time.Duration
	}{
		{"old.json", `{"todos": []}`, 0},
		{"middle.json", `{"todos": []}`, 100 * time.Millisecond},
		{"latest.json", `{"todos": []}`, 200 * time.Millisecond},
	}

	for i, f := range files {
		path := filepath.Join(tmpDir, f.name)

		err := os.WriteFile(path, []byte(f.content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", f.name, err)
		}

		modTime := time.Now().Add(-time.Duration(len(files)-i-1) * 100 * time.Millisecond)
		os.Chtimes(path, modTime, modTime)
		time.Sleep(f.delay)
	}

	t.Run("returns all JSON files sorted by time", func(t *testing.T) {
		result, err := getAllJSONFiles(tmpDir)
		if err != nil {
			t.Fatalf("getAllJSONFiles() error = %v", err)
		}

		if len(result) != 3 {
			t.Errorf("getAllJSONFiles() returned %d files, want 3", len(result))
		}

		// Первый файл должен быть latest.json (новый первый)
		if !strings.HasSuffix(result[0], "latest.json") {
			t.Errorf("getAllJSONFiles()[0] = %q, want latest.json", result[0])
		}
	})

	t.Run("empty directory returns error", func(t *testing.T) {
		emptyDir := t.TempDir()

		_, emptyErr := getAllJSONFiles(emptyDir)
		if emptyErr == nil {
			t.Error("getAllJSONFiles() expected error for empty directory")
		}
	})

	t.Run("ignores non-JSON files", func(t *testing.T) {
		mixedDir := t.TempDir()

		os.WriteFile(filepath.Join(mixedDir, "file.txt"), []byte("text"), 0644)
		os.WriteFile(filepath.Join(mixedDir, "data.json"), []byte(`{"todos": []}`), 0644)
		os.WriteFile(filepath.Join(mixedDir, "README.md"), []byte("# Readme"), 0644)

		result, err := getAllJSONFiles(mixedDir)
		if err != nil {
			t.Fatalf("getAllJSONFiles() error = %v", err)
		}

		if len(result) != 1 {
			t.Errorf("getAllJSONFiles() returned %d files, want 1", len(result))
		}

		if !strings.HasSuffix(result[0], "data.json") {
			t.Errorf("getAllJSONFiles()[0] = %q, want data.json", result[0])
		}
	})
}

func TestProcessSessionFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid session with all statuses", func(t *testing.T) {
		jsonContent := `{
			"sessionId": "test-session-123",
			"todos": [
				{"content": "Task 1", "activeForm": "Working on it", "status": "pending"},
				{"content": "Task 2", "activeForm": "", "status": "in_progress"},
				{"content": "Task 3", "activeForm": "Done", "status": "completed"}
			]
		}`

		path := filepath.Join(tmpDir, "test.json")
		err := os.WriteFile(path, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Создаём сервер без загрузки шаблона
		server := &Server{todoDir: tmpDir}

		result, err := server.processSessionFile(path)
		if err != nil {
			t.Fatalf("processSessionFile() error = %v", err)
		}

		if result.ID != "test-session-123" {
			t.Errorf("processSessionFile() ID = %q, want test-session-123", result.ID)
		}

		// Проверяем, что задачи распределены по массивам
		if len(result.Pending) != 1 || result.Pending[0].Content != "Task 1" {
			t.Error("processSessionFile() Pending should contain Task 1")
		}

		if len(result.InProgress) != 1 || result.InProgress[0].Content != "Task 2" {
			t.Error("processSessionFile() InProgress should contain Task 2")
		}

		if len(result.Completed) != 1 || result.Completed[0].Content != "Task 3" {
			t.Error("processSessionFile() Completed should contain Task 3")
		}

		// Проверяем счётчики
		if result.TaskCounts.Pending != 1 {
			t.Errorf("processSessionFile() Pending count = %d, want 1", result.TaskCounts.Pending)
		}
		if result.TaskCounts.InProgress != 1 {
			t.Errorf("processSessionFile() InProgress count = %d, want 1", result.TaskCounts.InProgress)
		}
		if result.TaskCounts.Completed != 1 {
			t.Errorf("processSessionFile() Completed count = %d, want 1", result.TaskCounts.Completed)
		}
	})

	t.Run("missing sessionId uses filename", func(t *testing.T) {
		jsonContent := `{"todos": []}`

		path := filepath.Join(tmpDir, "abc-123.json")
		err := os.WriteFile(path, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		server := &Server{todoDir: tmpDir}

		result, err := server.processSessionFile(path)
		if err != nil {
			t.Fatalf("processSessionFile() error = %v", err)
		}

		if result.ID != "abc-123" {
			t.Errorf("processSessionFile() ID = %q, want abc-123", result.ID)
		}
	})

	t.Run("empty content becomes No description", func(t *testing.T) {
		jsonContent := `{
			"sessionId": "test",
			"todos": [
				{"content": "", "status": "pending"}
			]
		}`

		path := filepath.Join(tmpDir, "empty.json")
		err := os.WriteFile(path, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		server := &Server{todoDir: tmpDir}

		result, err := server.processSessionFile(path)
		if err != nil {
			t.Fatalf("processSessionFile() error = %v", err)
		}

		if len(result.Pending) != 1 || result.Pending[0].Content != "No description" {
			t.Error("processSessionFile() should use 'No description' for empty content")
		}
	})

	t.Run("empty status defaults to pending", func(t *testing.T) {
		jsonContent := `{
			"sessionId": "test",
			"todos": [
				{"content": "Task", "status": ""}
			]
		}`

		path := filepath.Join(tmpDir, "default.json")
		err := os.WriteFile(path, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		server := &Server{todoDir: tmpDir}

		result, err := server.processSessionFile(path)
		if err != nil {
			t.Fatalf("processSessionFile() error = %v", err)
		}

		if len(result.Pending) != 1 || result.Pending[0].Content != "Task" {
			t.Error("processSessionFile() should default empty status to pending")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		jsonContent := `{invalid json}`

		path := filepath.Join(tmpDir, "invalid.json")
		err := os.WriteFile(path, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		server := &Server{todoDir: tmpDir}

		_, err = server.processSessionFile(path)
		if err == nil {
			t.Error("processSessionFile() expected error for invalid JSON")
		}
	})

	t.Run("non-existent file returns error", func(t *testing.T) {
		server := &Server{todoDir: tmpDir}

		_, err := server.processSessionFile("/non/existent/file.json")
		if err == nil {
			t.Error("processSessionFile() expected error for non-existent file")
		}
	})

	t.Run("completed only session is filtered", func(t *testing.T) {
		jsonContent := `{
			"sessionId": "completed-only",
			"todos": [
				{"content": "Task 1", "status": "completed"},
				{"content": "Task 2", "status": "completed"}
			]
		}`

		path := filepath.Join(tmpDir, "completed.json")
		err := os.WriteFile(path, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		server := &Server{todoDir: tmpDir}

		result, err := server.processSessionFile(path)
		if err != nil {
			t.Fatalf("processSessionFile() error = %v", err)
		}

		// Проверяем, что сессия имеет только completed задачи
		if len(result.Pending) > 0 {
			t.Error("Completed-only session should have empty Pending")
		}

		if len(result.InProgress) > 0 {
			t.Error("Completed-only session should have empty InProgress")
		}

		if len(result.Completed) != 2 {
			t.Error("Completed-only session should have 2 completed tasks")
		}

		// Проверяем статус сессии через determineSessionStatus
		status := server.determineSessionStatus(result)
		if status != "completed" {
			t.Errorf("determineSessionStatus() should return 'completed' for completed-only session, got %q", status)
		}
	})
}

func TestNewServer(t *testing.T) {
	// Получаем корень проекта (на два уровня выше от директории теста)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Находим корень проекта (где лежит go.mod)
	projectRoot := wd
	for !fileExists(filepath.Join(projectRoot, "go.mod")) {
		projectRoot = filepath.Dir(projectRoot)
		if projectRoot == "/" {
			t.Fatal("Could not find project root (go.mod not found)")
		}
	}

	// Меняем рабочую директорию на корень проекта
	t.Chdir(projectRoot)

	tmpDir := t.TempDir()

	// Создаём тестовый JSON-файл
	jsonContent := `{
		"sessionId": "test-session",
		"todos": [
			{"content": "Task 1", "status": "pending"}
		]
	}`
	err = os.WriteFile(filepath.Join(tmpDir, "test.json"), []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	t.Run("successful server creation", func(t *testing.T) {
		server, err := NewServer(tmpDir, time.Second*2, 5)
		if err != nil {
			t.Fatalf("NewServer() error = %v", err)
		}
		defer server.Stop()

		if server == nil {
			t.Error("NewServer() returned nil server")
		}
	})

	t.Run("invalid template path", func(t *testing.T) {
		// Временно переименовываем шаблон
		origPath := filepath.Join("templates", "kanban.html")
		tmpPath := filepath.Join("templates", "kanban.html.bak")

		// Проверяем, существует ли оригинальный файл
		if _, err := os.Stat(origPath); err == nil {
			// Переименовываем
			os.Rename(origPath, tmpPath) // Тест: игнорируем ошибку
			defer os.Rename(tmpPath, origPath) // Тест: игнорируем ошибку
		} else {
			t.Skip("Template file not found, skipping test")
		}

		_, err := NewServer(tmpDir, time.Second*2, 5)
		if err == nil {
			t.Error("NewServer() expected error for missing template")
		}
	})

	t.Run("non-existent todo directory", func(t *testing.T) {
		server, err := NewServer("/non/existent/path", time.Second*2, 5)
		if err != nil {
			// Ожидаем ошибку при загрузке кэша
			if !strings.Contains(err.Error(), "refresh sessions cache") {
				t.Errorf("NewServer() unexpected error = %v", err)
			}
		}
		if server != nil {
			server.Stop()
		}
	})
}

func TestLoadTemplate(t *testing.T) {
	// Меняем рабочую директорию на корень проекта
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot := wd
	for !fileExists(filepath.Join(projectRoot, "go.mod")) {
		projectRoot = filepath.Dir(projectRoot)
		if projectRoot == "/" {
			t.Fatal("Could not find project root (go.mod not found)")
		}
	}

	t.Chdir(projectRoot)

	tmpDir := t.TempDir()

	server := &Server{
		todoDir:           tmpDir,
		refreshInterval:   time.Second * 2,
		uiRefreshInterval: 5,
		stopRefresh:       make(chan struct{}),
	}

	t.Run("load existing template", func(t *testing.T) {
		err := server.loadTemplate()
		if err != nil {
			t.Errorf("loadTemplate() error = %v", err)
		}

		if server.template == nil {
			t.Error("loadTemplate() did not set template")
		}
	})

	t.Run("load non-existent template", func(t *testing.T) {
		// Временно переименовываем шаблон
		origPath := filepath.Join("templates", "kanban.html")
		tmpPath := filepath.Join("templates", "kanban.html.bak")

		if _, err := os.Stat(origPath); err == nil {
			os.Rename(origPath, tmpPath) // Тест: игнорируем ошибку
			defer os.Rename(tmpPath, origPath) // Тест: игнорируем ошибку
		} else {
			t.Skip("Template file not found, skipping test")
		}

		err := server.loadTemplate()
		if err == nil {
			t.Error("loadTemplate() expected error for missing template")
		}
	})
}

func TestRefreshSessionsCache(t *testing.T) {
	// Меняем рабочую директорию на корень проекта
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot := wd
	for !fileExists(filepath.Join(projectRoot, "go.mod")) {
		projectRoot = filepath.Dir(projectRoot)
		if projectRoot == "/" {
			t.Fatal("Could not find project root (go.mod not found)")
		}
	}

	t.Chdir(projectRoot)

	tmpDir := t.TempDir()

	t.Run("cache with active sessions", func(t *testing.T) {
		jsonContent := `{
			"sessionId": "active-session",
			"todos": [
				{"content": "Task 1", "status": "in_progress"}
			]
		}`
		err = os.WriteFile(filepath.Join(tmpDir, "active.json"), []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		server := &Server{
			todoDir:           tmpDir,
			refreshInterval:   time.Second * 2,
			uiRefreshInterval: 5,
			stopRefresh:       make(chan struct{}),
		}

		err = server.loadTemplate()
		if err != nil {
			t.Fatalf("loadTemplate() error = %v", err)
		}

		err = server.refreshSessionsCache()
		if err != nil {
			t.Errorf("refreshSessionsCache() error = %v", err)
		}

		server.cacheMu.RLock()
		cacheLen := len(server.sessionsCache)
		if cacheLen != 1 {
			server.cacheMu.RUnlock()
			t.Errorf("refreshSessionsCache() expected 1 session, got %d", cacheLen)
		} else {
			// Проверяем новый статус сессии (active = есть in_progress)
			session := server.sessionsCache[0]
			if session.Status != "active" {
				server.cacheMu.RUnlock()
				t.Errorf("refreshSessionsCache() expected status 'active', got %q", session.Status)
			}
			// Проверяем счётчики
			if session.TaskCounts.InProgress != 1 {
				server.cacheMu.RUnlock()
				t.Errorf("refreshSessionsCache() expected InProgress count 1, got %d", session.TaskCounts.InProgress)
			}
			server.cacheMu.RUnlock()
		}
	})

	t.Run("cache with inactive sessions (pending only)", func(t *testing.T) {
		tmpDir3 := t.TempDir()

		jsonContent := `{
			"sessionId": "inactive-session",
			"todos": [
				{"content": "Task 1", "status": "pending"},
				{"content": "Task 2", "status": "pending"}
			]
		}`
		err := os.WriteFile(filepath.Join(tmpDir3, "inactive.json"), []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		server := &Server{
			todoDir:           tmpDir3,
			refreshInterval:   time.Second * 2,
			uiRefreshInterval: 5,
			stopRefresh:       make(chan struct{}),
		}

		err = server.loadTemplate()
		if err != nil {
			t.Fatalf("loadTemplate() error = %v", err)
		}

		err = server.refreshSessionsCache()
		if err != nil {
			t.Errorf("refreshSessionsCache() error = %v", err)
		}

		server.cacheMu.RLock()
		cacheLen := len(server.sessionsCache)
		if cacheLen != 1 {
			server.cacheMu.RUnlock()
			t.Errorf("refreshSessionsCache() expected 1 session, got %d", cacheLen)
		} else {
			// Проверяем, что сессия имеет статус inactive (только pending, без in_progress)
			session := server.sessionsCache[0]
			if session.Status != "inactive" {
				server.cacheMu.RUnlock()
				t.Errorf("refreshSessionsCache() expected status 'inactive', got %q", session.Status)
			}
			if session.TaskCounts.Pending != 2 {
				server.cacheMu.RUnlock()
				t.Errorf("refreshSessionsCache() expected Pending count 2, got %d", session.TaskCounts.Pending)
			}
			server.cacheMu.RUnlock()
		}
	})

	t.Run("cache filters completed-only sessions", func(t *testing.T) {
		tmpDir2 := t.TempDir()

		jsonContent := `{
			"sessionId": "completed-session",
			"todos": [
				{"content": "Task 1", "status": "completed"}
			]
		}`
		err := os.WriteFile(filepath.Join(tmpDir2, "completed.json"), []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		server := &Server{
			todoDir:           tmpDir2,
			refreshInterval:   time.Second * 2,
			uiRefreshInterval: 5,
			stopRefresh:       make(chan struct{}),
		}

		err = server.loadTemplate()
		if err != nil {
			t.Fatalf("loadTemplate() error = %v", err)
		}

		err = server.refreshSessionsCache()
		if err != nil {
			t.Errorf("refreshSessionsCache() error = %v", err)
		}

		server.cacheMu.RLock()
		cacheLen := len(server.sessionsCache)
		if cacheLen != 1 {
			server.cacheMu.RUnlock()
			t.Errorf("refreshSessionsCache() expected 1 session (completed are now stored), got %d", cacheLen)
		} else {
			// Проверяем, что сессия имеет статус completed
			session := server.sessionsCache[0]
			if session.Status != "completed" {
				server.cacheMu.RUnlock()
				t.Errorf("refreshSessionsCache() expected status 'completed', got %q", session.Status)
			}
			server.cacheMu.RUnlock()
		}
	})

	t.Run("cache with non-existent directory", func(t *testing.T) {
		server := &Server{
			todoDir:           "/non/existent/path",
			refreshInterval:   time.Second * 2,
			uiRefreshInterval: 5,
			stopRefresh:       make(chan struct{}),
		}

		err := server.refreshSessionsCache()
		if err == nil {
			t.Error("refreshSessionsCache() expected error for non-existent directory")
		}
	})
}

func TestKanbanHandler(t *testing.T) {
	// Меняем рабочую директорию на корень проекта
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot := wd
	for !fileExists(filepath.Join(projectRoot, "go.mod")) {
		projectRoot = filepath.Dir(projectRoot)
		if projectRoot == "/" {
			t.Fatal("Could not find project root (go.mod not found)")
		}
	}

	t.Chdir(projectRoot)

	tmpDir := t.TempDir()

	t.Run("handler returns active sessions", func(t *testing.T) {
		jsonContent := `{
			"sessionId": "test-session",
			"todos": [
				{"content": "Task 1", "status": "pending"}
			]
		}`
		err = os.WriteFile(filepath.Join(tmpDir, "test.json"), []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		server, err := NewServer(tmpDir, time.Second*2, 5)
		if err != nil {
			t.Fatalf("NewServer() error = %v", err)
		}
		defer server.Stop()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		server.KanbanHandler(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("KanbanHandler() status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body := w.Body.String()
		if !strings.Contains(body, "test-session") {
			t.Error("KanbanHandler() response should contain session ID")
		}
	})

	t.Run("handler returns message for non-existent directory", func(t *testing.T) {
		server := &Server{
			todoDir:           "/non/existent/path",
			refreshInterval:   time.Second * 2,
			uiRefreshInterval: 5,
			stopRefresh:       make(chan struct{}),
		}

		err := server.loadTemplate()
		if err != nil {
			t.Fatalf("loadTemplate() error = %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		server.KanbanHandler(w, req)

		body := w.Body.String()
		if !strings.Contains(body, "not found") {
			t.Error("KanbanHandler() should return 'not found' message")
		}
	})

	t.Run("handler returns message for no active sessions", func(t *testing.T) {
		tmpDir2 := t.TempDir()

		jsonContent := `{
			"sessionId": "completed-session",
			"todos": [
				{"content": "Task 1", "status": "completed"}
			]
		}`
		err := os.WriteFile(filepath.Join(tmpDir2, "completed.json"), []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		server, err := NewServer(tmpDir2, time.Second*2, 5)
		if err != nil {
			t.Fatalf("NewServer() error = %v", err)
		}
		defer server.Stop()

		// Ждём обновления кэша
		time.Sleep(time.Millisecond * 100)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		server.KanbanHandler(w, req)

		body := w.Body.String()
		// Проверяем, что есть сообщение о пустых секциях
		if !strings.Contains(body, "Нет сессий") {
			t.Error("KanbanHandler() should return 'Нет сессий' message for empty directory")
		}
	})
}

func TestStop(t *testing.T) {
	// Меняем рабочую директорию на корень проекта
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot := wd
	for !fileExists(filepath.Join(projectRoot, "go.mod")) {
		projectRoot = filepath.Dir(projectRoot)
		if projectRoot == "/" {
			t.Fatal("Could not find project root (go.mod not found)")
		}
	}

	t.Chdir(projectRoot)

	tmpDir := t.TempDir()

	// Создаём тестовый JSON-файл, чтобы NewServer не упал при загрузке кэша
	jsonContent := `{
		"sessionId": "test-session",
		"todos": [
			{"content": "Task 1", "status": "pending"}
		]
	}`
	err = os.WriteFile(filepath.Join(tmpDir, "test.json"), []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	server, err := NewServer(tmpDir, time.Second*2, 5)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Stop должен закрывать канал stopRefresh
	server.Stop()

	// Проверяем, что канал закрыт
	select {
	case _, ok := <-server.stopRefresh:
		if ok {
			t.Error("Stop() did not close stopRefresh channel")
		}
	default:
		t.Error("Stop() channel should be closed")
	}
}

func TestDetermineSessionStatus(t *testing.T) {
	tmpDir := t.TempDir()

	server := &Server{
		todoDir:           tmpDir,
		refreshInterval:   time.Second * 2,
		uiRefreshInterval: 5,
		stopRefresh:       make(chan struct{}),
	}

	t.Run("session with only pending tasks returns inactive", func(t *testing.T) {
		session := SessionData{
			Pending:    []Task{{Content: "Task", Status: "pending"}},
			InProgress: []Task{},
			Completed:  []Task{},
		}

		status := server.determineSessionStatus(session)
		if status != "inactive" {
			t.Errorf("determineSessionStatus() expected 'inactive', got %q", status)
		}
	})

	t.Run("session with in_progress tasks returns active", func(t *testing.T) {
		session := SessionData{
			Pending:    []Task{},
			InProgress: []Task{{Content: "Task", Status: "in_progress"}},
			Completed:  []Task{},
		}

		status := server.determineSessionStatus(session)
		if status != "active" {
			t.Errorf("determineSessionStatus() expected 'active', got %q", status)
		}
	})

	t.Run("session with pending and in_progress returns active", func(t *testing.T) {
		session := SessionData{
			Pending:    []Task{{Content: "Task", Status: "pending"}},
			InProgress: []Task{{Content: "Task", Status: "in_progress"}},
			Completed:  []Task{},
		}

		status := server.determineSessionStatus(session)
		if status != "active" {
			t.Errorf("determineSessionStatus() expected 'active', got %q", status)
		}
	})

	t.Run("session with only completed tasks returns completed", func(t *testing.T) {
		session := SessionData{
			Pending:    []Task{},
			InProgress: []Task{},
			Completed:  []Task{{Content: "Task", Status: "completed"}},
		}

		status := server.determineSessionStatus(session)
		if status != "completed" {
			t.Errorf("determineSessionStatus() expected 'completed', got %q", status)
		}
	})

	t.Run("empty session returns inactive", func(t *testing.T) {
		session := SessionData{
			Pending:    []Task{},
			InProgress: []Task{},
			Completed:  []Task{},
		}

		status := server.determineSessionStatus(session)
		if status != "inactive" {
			t.Errorf("determineSessionStatus() expected 'inactive', got %q", status)
		}
	})
}
