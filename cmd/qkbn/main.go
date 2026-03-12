package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"

	qkbnhttp "github.com/jtprogru/qkbn/internal/qkbnhttp"
)

const (
	defaultPort    = 9090
	defaultTodoDir = "~/.qwen/todos/"
	minPort        = 1
	maxPort        = 65535
)

func validatePort(port int) error {
	if port < minPort || port > maxPort {
		return fmt.Errorf("port must be between %d and %d", minPort, maxPort)
	}

	return nil
}

func main() {
	port := flag.Int("port", defaultPort, "Port to run the web server on")
	flag.IntVar(port, "p", defaultPort, "Shorthand for -port")

	todosDir := flag.String("todos-dir", defaultTodoDir, "Directory containing Qwen-code todo JSON files")
	flag.StringVar(todosDir, "d", defaultTodoDir, "Shorthand for -todos-dir")

	flag.Parse()

	// Валидация порта
	portErr := validatePort(*port)
	if portErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", portErr)
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
	server, err := qkbnhttp.NewServer(*todosDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	http.HandleFunc("/", server.KanbanHandler)

	addr := ":" + strconv.Itoa(*port)

	fmt.Printf("🚀 Local Kanban is running! Open http://localhost:%d in your browser.\n", *port)
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
