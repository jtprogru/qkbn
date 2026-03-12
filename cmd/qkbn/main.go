package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	qkbnhttp "github.com/jtprogru/qkbn/internal/qkbnhttp"
)

const (
	defaultPort         = 9090
	defaultTodoDir      = "~/.qwen/todos/"
	defaultRefreshSec   = 120 // 2 минуты
	minPort             = 1
	maxPort             = 65535
	minRefreshInterval  = 10 * time.Second
	defaultRefreshInterval = time.Duration(defaultRefreshSec) * time.Second
)

func validatePort(port int) error {
	if port < minPort || port > maxPort {
		return fmt.Errorf("port must be between %d and %d", minPort, maxPort)
	}

	return nil
}

func validateRefreshInterval(d time.Duration) error {
	if d < minRefreshInterval {
		return fmt.Errorf("refresh interval must be at least %v", minRefreshInterval)
	}

	return nil
}

func main() {
	port := flag.Int("port", defaultPort, "Port to run the web server on")
	flag.IntVar(port, "p", defaultPort, "Shorthand for -port")

	todosDir := flag.String("todos-dir", defaultTodoDir, "Directory containing Qwen-code todo JSON files")
	flag.StringVar(todosDir, "d", defaultTodoDir, "Shorthand for -todos-dir")

	refreshInterval := flag.Duration("refresh-interval", defaultRefreshInterval, "Interval for refreshing sessions cache (e.g., 2m, 120s)")
	flag.DurationVar(refreshInterval, "r", defaultRefreshInterval, "Shorthand for -refresh-interval")

	flag.Parse()

	// Валидация порта
	portErr := validatePort(*port)
	if portErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", portErr)
		os.Exit(1)
	}

	// Валидация интервала обновления
	refreshErr := validateRefreshInterval(*refreshInterval)
	if refreshErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", refreshErr)
		os.Exit(1)
	}

	// Проверка директории
	expandedDir := expandPath(*todosDir)

	_, statErr := os.Stat(expandedDir)
	if os.IsNotExist(statErr) {
		fmt.Fprintf(os.Stderr, "Error: directory %s does not exist\n", expandedDir)
		os.Exit(1)
	}

	// Создаём сервер
	server, err := qkbnhttp.NewServer(*todosDir, *refreshInterval)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	// Обрабатываем сигналы для корректного завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		server.Stop()
		os.Exit(0)
	}()

	http.HandleFunc("/", server.KanbanHandler)

	addr := ":" + strconv.Itoa(*port)

	fmt.Printf("🚀 Local Kanban is running! Open http://localhost:%d in your browser.\n", *port)
	fmt.Printf("📁 Reading sessions from: %s\n", expandedDir)
	fmt.Printf("🔄 Refresh interval: %v\n", *refreshInterval)
	//nolint:gosec,noinlineerr // Локальный сервер без внешних подключений, таймауты не критичны
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}

func expandPath(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		return home
	}

	if len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		return home + path[1:]
	}

	return path
}
