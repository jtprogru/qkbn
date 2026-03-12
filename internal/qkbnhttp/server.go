package qkbnhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Server представляет HTTP-сервер с Kanban-доской.
type Server struct {
	todoDir  string
	template *template.Template
	mu       sync.RWMutex
}

// Task представляет задачу из JSON-файла.
type Task struct {
	Content    string `json:"content"`
	ActiveForm string `json:"activeForm"`
	Status     string `json:"status"`
}

// Data представляет структуру JSON-файла с задачами.
type Data struct {
	Todos []Task `json:"todos"`
}

// PageData представляет данные для шаблона страницы.
type PageData struct {
	Pending    template.HTML
	InProgress template.HTML
	Completed  template.HTML
}

// NewServer создаёт новый сервер с заданной директорией задач.
func NewServer(todoDir string) (*Server, error) {
	s := &Server{
		todoDir: todoDir,
	}

	if err := s.loadTemplate(); err != nil {
		return nil, fmt.Errorf("load template: %w", err)
	}

	return s, nil
}

// KanbanHandler обрабатывает запросы к главной странице.
func (s *Server) KanbanHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Проверяем существование директории
	expandedDir := expandPath(s.todoDir)

	_, err := os.Stat(expandedDir)
	if os.IsNotExist(err) {
		fmt.Fprintf(w, "Directory %s not found. Run qwen-code first.", expandedDir)
		return
	}

	// Получаем самый свежий файл
	latestFile, err := getLatestFile(s.todoDir)
	if err != nil {
		fmt.Fprintf(w, "No active qwen-code tasks found.")
		return
	}

	// Читаем и парсим JSON
	data, err := os.ReadFile(latestFile)
	if err != nil {
		fmt.Fprintf(w, "Error reading file: %v", err)
		return
	}

	var jsonData Data

	err = json.Unmarshal(data, &jsonData)
	if err != nil {
		fmt.Fprintf(w, "Error reading JSON: %v", err)
		return
	}

	var pending, inProgress, completed strings.Builder

	// Распределяем карточки по колонкам
	for _, task := range jsonData.Todos {
		content := task.Content
		if content == "" {
			content = "No description"
		}

		action := task.ActiveForm

		status := task.Status
		if status == "" {
			status = "pending"
		}

		cardHTML := fmt.Sprintf("<div class='card'><div class='card-title'>%s</div>", content)

		if action != "" {
			cardHTML += fmt.Sprintf("<div class='card-action'>⚡ %s</div>", action)
		}

		cardHTML += "</div>"

		switch status {
		case "pending":
			pending.WriteString(cardHTML)
		case "in_progress":
			inProgress.WriteString(cardHTML)
		case "completed":
			completed.WriteString(cardHTML)
		}
	}

	s.mu.RLock()
	tmpl := s.template
	s.mu.RUnlock()

	pageData := PageData{
		//nolint:gosec // HTML генерируется из доверенного источника (JSON файлы пользователя)
		Pending:    template.HTML(pending.String()),
		//nolint:gosec // HTML генерируется из доверенного источника (JSON файлы пользователя)
		InProgress: template.HTML(inProgress.String()),
		//nolint:gosec // HTML генерируется из доверенного источника (JSON файлы пользователя)
		Completed:  template.HTML(completed.String()),
	}

	err = tmpl.Execute(w, pageData)
	if err != nil {
		fmt.Fprintf(w, "Execution error: %v", err)
	}
}

// loadTemplate загружает HTML-шаблон из файловой системы.
func (s *Server) loadTemplate() error {
	// Путь относительно корня проекта
	tmplPath := filepath.Join("templates", "kanban.html")

	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.template = tmpl
	return nil
}

// expandPath раскрывает ~ до домашней директории.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		return filepath.Join(home, path[2:])
	}

	return path
}

// getLatestFile возвращает путь к последнему изменённому JSON-файлу.
func getLatestFile(dir string) (string, error) {
	expandedDir := expandPath(dir)

	files, err := os.ReadDir(expandedDir)
	if err != nil {
		return "", err
	}

	var jsonFiles []os.DirEntry

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			jsonFiles = append(jsonFiles, file)
		}
	}

	if len(jsonFiles) == 0 {
		return "", errors.New("no JSON files found")
	}

	// Сортируем по времени модификации
	sort.Slice(jsonFiles, func(i, j int) bool {
		infoI, _ := jsonFiles[i].Info()
		infoJ, _ := jsonFiles[j].Info()

		return infoI.ModTime().After(infoJ.ModTime())
	})

	return filepath.Join(expandedDir, jsonFiles[0].Name()), nil
}
