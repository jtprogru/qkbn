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
	ID           string     `json:"id"`
	Pending      []Task     `json:"pending"`
	InProgress   []Task     `json:"inProgress"`
	Completed    []Task     `json:"completed"`
	Status       string     `json:"status"` // "active", "inactive", "completed"
	TaskCounts   TaskCounts `json:"taskCounts"`
	PendingHTML  template.HTML
	InProgressHTML template.HTML
	CompletedHTML  template.HTML
}

// TaskCounts хранит счётчики задач по колонкам.
type TaskCounts struct {
	Pending    int
	InProgress int
	Completed  int
}

// SessionsAPIResponse представляет ответ API для сессий.
type SessionsAPIResponse struct {
	ActiveSessions    []SessionData `json:"activeSessions"`
	InactiveSessions  []SessionData `json:"inactiveSessions"`
	CompletedSessions []SessionData `json:"completedSessions"`
	LastUpdated       string        `json:"lastUpdated"`
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

// SessionsAPIHandler обрабатывает запросы к API сессий.
func (s *Server) SessionsAPIHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

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

	response := SessionsAPIResponse{
		ActiveSessions:    activeSessions,
		InactiveSessions:  inactiveSessions,
		CompletedSessions: completedSessions,
		LastUpdated:       time.Now().Format(time.RFC3339),
	}

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error": "%v"}`, err)
	}
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

		// Определяем статус сессии
		status := s.determineSessionStatus(sessionData)
		sessionData.Status = status

		// Сохраняем все сессии (группировка будет в хендлере)
		sessions = append(sessions, sessionData)
	}

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.sessionsCache = sessions
	return nil
}

// determineSessionStatus определяет статус сессии на основе задач.
func (s *Server) determineSessionStatus(session SessionData) string {
	hasPending := len(session.Pending) > 0
	hasInProgress := len(session.InProgress) > 0
	hasCompleted := len(session.Completed) > 0

	switch {
	// Active: есть задачи в работе
	case hasInProgress:
		return "active"
	// Inactive: есть только pending задачи (нет in_progress)
	case hasPending && !hasInProgress:
		return "inactive"
	// Completed: только завершенные задачи
	case hasCompleted && !hasPending && !hasInProgress:
		return "completed"
	// Пустая сессия
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

	var pendingHTML, inProgressHTML, completedHTML strings.Builder
	var counts TaskCounts
	// Инициализируем пустые срезы, чтобы они не стали null в JSON
	pendingTasks := make([]Task, 0)
	inProgressTasks := make([]Task, 0)
	completedTasks := make([]Task, 0)

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

		// Создаём копию задачи для JSON
		taskCopy := Task{
			Content:    content,
			ActiveForm: action,
			Status:     status,
		}

		cardHTML := fmt.Sprintf("<div class='card'><div class='card-title'>%s</div>", content)

		if action != "" {
			cardHTML += fmt.Sprintf("<div class='card-action'>⚡ %s</div>", action)
		}

		cardHTML += "</div>"

		switch status {
		case "pending":
			pendingHTML.WriteString(cardHTML)
			counts.Pending++
			pendingTasks = append(pendingTasks, taskCopy)
		case "in_progress":
			inProgressHTML.WriteString(cardHTML)
			counts.InProgress++
			inProgressTasks = append(inProgressTasks, taskCopy)
		case "completed":
			completedHTML.WriteString(cardHTML)
			counts.Completed++
			completedTasks = append(completedTasks, taskCopy)
		}
	}

	sessionID := jsonData.SessionID
	if sessionID == "" {
		// Используем имя файла как ID сессии
		sessionID = filepath.Base(filePath)
		sessionID = strings.TrimSuffix(sessionID, ".json")
	}

	return SessionData{
		ID:             sessionID,
		Pending:        pendingTasks,
		InProgress:     inProgressTasks,
		Completed:      completedTasks,
		PendingHTML:    template.HTML(pendingHTML.String()),    //nolint:gosec // Доверенный источник
		InProgressHTML: template.HTML(inProgressHTML.String()), //nolint:gosec // Доверенный источник
		CompletedHTML:  template.HTML(completedHTML.String()),  //nolint:gosec // Доверенный источник
		TaskCounts:     counts,
	}, nil
}
