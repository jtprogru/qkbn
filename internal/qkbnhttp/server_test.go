package qkbnhttp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

		// Проверяем, что карточки распределены по колонкам
		if !strings.Contains(string(result.Pending), "Task 1") {
			t.Error("processSessionFile() Pending should contain Task 1")
		}

		if !strings.Contains(string(result.InProgress), "Task 2") {
			t.Error("processSessionFile() InProgress should contain Task 2")
		}

		if !strings.Contains(string(result.Completed), "Task 3") {
			t.Error("processSessionFile() Completed should contain Task 3")
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

		if !strings.Contains(string(result.Pending), "No description") {
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

		if !strings.Contains(string(result.Pending), "Task") {
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
		if len(string(result.Pending)) > 0 {
			t.Error("Completed-only session should have empty Pending")
		}

		if len(string(result.InProgress)) > 0 {
			t.Error("Completed-only session should have empty InProgress")
		}

		// hasActiveTasks должен вернуть false
		if server.hasActiveTasks(result) {
			t.Error("hasActiveTasks() should return false for completed-only session")
		}
	})
}
