package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{
			name:    "valid default port",
			port:    9090,
			wantErr: false,
		},
		{
			name:    "valid min port",
			port:    1,
			wantErr: false,
		},
		{
			name:    "valid max port",
			port:    65535,
			wantErr: false,
		},
		{
			name:    "port below minimum",
			port:    0,
			wantErr: true,
		},
		{
			name:    "port above maximum",
			port:    65536,
			wantErr: true,
		},
		{
			name:    "negative port",
			port:    -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port)

			if tt.wantErr && err == nil {
				t.Errorf("validatePort(%d) expected error, got nil", tt.port)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("validatePort(%d) unexpected error = %v", tt.port, err)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHome bool // true если путь должен содержать домашнюю директорию
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
	// Создаём временную директорию
	tmpDir := t.TempDir()

	// Создаём тестовые JSON файлы с разным временем модификации
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
		// Устанавливаем разное время модификации
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

func TestTaskStatusDistribution(t *testing.T) {
	// Тест на правильное распределение задач по статусам
	tasks := []Task{
		{Content: "Task 1", ActiveForm: "Doing", Status: "pending"},
		{Content: "Task 2", ActiveForm: "Working", Status: "in_progress"},
		{Content: "Task 3", ActiveForm: "", Status: "completed"},
		{Content: "Task 4", ActiveForm: "Started", Status: ""}, // должен стать pending
	}

	var pending, inProgress, completed int

	for _, task := range tasks {
		status := task.Status
		if status == "" {
			status = "pending"
		}

		switch status {
		case "pending":
			pending++
		case "in_progress":
			inProgress++
		case "completed":
			completed++
		}
	}

	if pending != 2 {
		t.Errorf("Expected 2 pending tasks, got %d", pending)
	}

	if inProgress != 1 {
		t.Errorf("Expected 1 in_progress task, got %d", inProgress)
	}

	if completed != 1 {
		t.Errorf("Expected 1 completed task, got %d", completed)
	}
}

func TestGetLatestFile_IgnoresNonJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Создаём смешанные файлы
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
