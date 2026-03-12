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
	"time"
)

// Server представляет HTTP-сервер с Kanban-доской.
type Server struct {
	todoDir           string
	template          *template.Template
	mu                sync.RWMutex
	sessionsCache     []SessionData
	cacheMu           sync.RWMutex
	refreshInterval   time.Duration
	uiRefreshInterval int
	stopRefresh       chan struct{}
}

// Task представляет задачу из JSON-файла.
type Task struct {
	Content    string `json:"content"`
	ActiveForm string `json:"activeForm"`
	Status     string `json:"status"`
}

// Data представляет структуру JSON-файла с задачами.
type Data struct {
	Todos     []Task `json:"todos"`
	SessionID string `json:"sessionId"`
}

// SessionData представляет данные одной сессии для отображения.
type SessionData struct {
	ID           string
	Pending      template.HTML
	InProgress   template.HTML
	Completed    template.HTML
	Status       string // "active", "inactive", "completed"
	TaskCounts   TaskCounts
}

// TaskCounts хранит счётчики задач по колонкам.
type TaskCounts struct {
	Pending    int
	InProgress int
	Completed  int
}

// PageData представляет данные для шаблона страницы.
type PageData struct {
	Sessions          []SessionData
	ActiveSessions    []SessionData // pending OR in_progress
	InactiveSessions  []SessionData // only pending, no in_progress
	CompletedSessions []SessionData // only completed
	UIRefreshInterval int
}

// NewServer создаёт новый сервер с заданной директорией задач.
func NewServer(todoDir string, refreshInterval time.Duration, uiRefreshInterval int) (*Server, error) {
	s := &Server{
		todoDir:           todoDir,
		refreshInterval:   refreshInterval,
		uiRefreshInterval: uiRefreshInterval,
		stopRefresh:       make(chan struct{}),
	}

	if err := s.loadTemplate(); err != nil {
		return nil, fmt.Errorf("load template: %w", err)
	}

	// Загружаем кэш при старте
	if err := s.refreshSessionsCache(); err != nil {
		return nil, fmt.Errorf("refresh sessions cache: %w", err)
	}

	// Запускаем фоновое обновление
	go s.startRefreshLoop()

	return s, nil
}

// KanbanHandler обрабатывает запросы к главной странице.
func (s *Server) KanbanHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Проверяем существование директории
	expandedDir := expandPath(s.todoDir)

	_, err := os.Stat(expandedDir) //nolint:gosec // Локальный сервер, path контролируется пользователем
	if os.IsNotExist(err) {
		fmt.Fprintf(w, "Directory %s not found. Run qwen-code first.", expandedDir) //nolint:gosec // Локальный сервер, XSS не применим
		return
	}

	// Получаем сессии из кэша
	s.cacheMu.RLock()
	sessions := s.sessionsCache
	s.cacheMu.RUnlock()

	// Группируем сессии по статусу
	var activeSessions, inactiveSessions, completedSessions []SessionData
	for _, session := range sessions {
		switch session.Status {
		case "active":
			activeSessions = append(activeSessions, session)
		case "inactive":
			inactiveSessions = append(inactiveSessions, session)
		case "completed":
			completedSessions = append(completedSessions, session)
		}
	}

	// Если совсем нет сессий
	if len(activeSessions) == 0 && len(inactiveSessions) == 0 && len(completedSessions) == 0 {
		fmt.Fprintf(w, "No sessions found.")
		return
	}

	s.mu.RLock()
	tmpl := s.template
	s.mu.RUnlock()

	pageData := PageData{
		Sessions:          sessions,
		ActiveSessions:    activeSessions,
		InactiveSessions:  inactiveSessions,
		CompletedSessions: completedSessions,
		UIRefreshInterval: s.uiRefreshInterval,
	}

	err = tmpl.Execute(w, pageData)
	if err != nil {
		fmt.Fprintf(w, "Execution error: %v", err)
	}
}

// Stop останавливает фоновое обновление кэша.
func (s *Server) Stop() {
	close(s.stopRefresh)
}

// startRefreshLoop периодически обновляет кэш сессий.
func (s *Server) startRefreshLoop() {
	ticker := time.NewTicker(s.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = s.refreshSessionsCache() // Игнорируем ошибки, чтобы не спамить в логах
		case <-s.stopRefresh:
			return
		}
	}
}

// refreshSessionsCache обновляет кэш сессий.
func (s *Server) refreshSessionsCache() error {
	jsonFiles, err := getAllJSONFiles(s.todoDir)
	if err != nil {
		return err
	}

	var sessions []SessionData

	for _, file := range jsonFiles {
		sessionData, err := s.processSessionFile(file)
		if err != nil {
			continue // Пропускаем проблемные файлы
		}

		// Определяем статус сессии и фильтруем
		status := s.determineSessionStatus(sessionData)
		sessionData.Status = status

		// Пропускаем сессии без активных задач (только completed)
		if status == "completed" {
			continue
		}

		sessions = append(sessions, sessionData)
	}

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.sessionsCache = sessions
	return nil
}

// determineSessionStatus определяет статус сессии на основе задач.
func (s *Server) determineSessionStatus(session SessionData) string {
	hasPending := len(string(session.Pending)) > 0
	hasInProgress := len(string(session.InProgress)) > 0
	hasCompleted := len(string(session.Completed)) > 0

	switch {
	case hasInProgress || hasPending:
		return "active"
	case hasPending && !hasInProgress:
		return "inactive"
	case hasCompleted && !hasPending && !hasInProgress:
		return "completed"
	default:
		return "inactive"
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

// getAllJSONFiles возвращает все JSON-файлы, отсортированные по времени (новые первыми).
func getAllJSONFiles(dir string) ([]string, error) {
	expandedDir := expandPath(dir)

	files, err := os.ReadDir(expandedDir)
	if err != nil {
		return nil, err
	}

	var jsonFiles []os.DirEntry

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			jsonFiles = append(jsonFiles, file)
		}
	}

	if len(jsonFiles) == 0 {
		return nil, errors.New("no JSON files found")
	}

	// Сортируем по времени модификации (новые первыми)
	sort.Slice(jsonFiles, func(i, j int) bool {
		infoI, _ := jsonFiles[i].Info()
		infoJ, _ := jsonFiles[j].Info()

		return infoI.ModTime().After(infoJ.ModTime())
	})

	paths := make([]string, len(jsonFiles))

	for i, file := range jsonFiles {
		paths[i] = filepath.Join(expandedDir, file.Name())
	}

	return paths, nil
}

// processSessionFile обрабатывает один JSON-файл сессии и возвращает данные для отображения.
func (s *Server) processSessionFile(filePath string) (SessionData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return SessionData{}, err
	}

	var jsonData Data

	err = json.Unmarshal(data, &jsonData)
	if err != nil {
		return SessionData{}, err
	}

	var pending, inProgress, completed strings.Builder
	var counts TaskCounts

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
			counts.Pending++
		case "in_progress":
			inProgress.WriteString(cardHTML)
			counts.InProgress++
		case "completed":
			completed.WriteString(cardHTML)
			counts.Completed++
		}
	}

	sessionID := jsonData.SessionID
	if sessionID == "" {
		// Используем имя файла как ID сессии
		sessionID = filepath.Base(filePath)
		sessionID = strings.TrimSuffix(sessionID, ".json")
	}

	return SessionData{
		ID:         sessionID,
		Pending:    template.HTML(pending.String()),    //nolint:gosec // Доверенный источник
		InProgress: template.HTML(inProgress.String()), //nolint:gosec // Доверенный источник
		Completed:  template.HTML(completed.String()),  //nolint:gosec // Доверенный источник
		TaskCounts: counts,
	}, nil
}
