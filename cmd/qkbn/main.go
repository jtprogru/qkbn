package main

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
)

const (
    todoDir = "~/.qwen/todos/"
    htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>Qwen-code Kanban</title>
    <meta http-equiv="refresh" content="5"> <!-- Автообновление каждые 2 секунды -->
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #f4f5f7; padding: 20px; }
        .header { text-align: center; color: #333; margin-bottom: 20px; }
        .board { display: flex; gap: 20px; max-width: 1200px; margin: 0 auto; }
        .column { background: #ebecf0; border-radius: 8px; width: 33.33%; padding: 15px; }
        .column h2 { font-size: 14px; color: #5e6c84; text-transform: uppercase; margin-top: 0; }
        .card { background: #fff; padding: 12px; margin-bottom: 10px; border-radius: 4px; box-shadow: 0 1px 2px rgba(9,30,66,0.25); }
        .card-title { font-size: 14px; color: #172b4d; font-weight: 500; }
        .card-action { font-size: 12px; color: #0052cc; margin-top: 8px; font-style: italic; }
    </style>
</head>
<body>
    <div class="header"><h2>Qwen-code Active Session</h2></div>
    <div class="board">
        <div class="column"><h2>TODO (Pending)</h2>{{.Pending}}</div>
        <div class="column"><h2>IN PROGRESS</h2>{{.InProgress}}</div>
        <div class="column"><h2>DONE (Completed)</h2>{{.Completed}}</div>
    </div>
</body>
</html>
`
)

type Task struct {
    Content    string `json:"content"`
    ActiveForm string `json:"activeForm"`
    Status     string `json:"status"`
}

type Data struct {
    Todos []Task `json:"todos"`
}

type PageData struct {
    Pending   template.HTML
    InProgress template.HTML
    Completed  template.HTML
}

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

func kanbanHandler(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")

    // Проверяем существование директории
    expandedDir := expandPath(todoDir)

    _, err := os.Stat(expandedDir)
    if os.IsNotExist(err) {
        fmt.Fprintf(w, "Directory ~/.qwen/todos/ not found. Run qwen-code first.")
        return
    }

    // Получаем самый свежий файл
    latestFile, err := getLatestFile(todoDir)
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

    // Генерируем HTML
    tmpl, err := template.New("kanban").Parse(htmlTemplate)
    if err != nil {
        fmt.Fprintf(w, "Template error: %v", err)
        return
    }

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

func main() {
    http.HandleFunc("/", kanbanHandler)

    fmt.Println("🚀 Local Kanban is running! Open http://localhost:9090 in your browser.")
    //nolint:gosec,noinlineerr // Локальный сервер без внешних подключений, таймауты не критичны
    if err := http.ListenAndServe(":9090", nil); err != nil {
        fmt.Printf("Server error: %v\n", err)
        os.Exit(1)
    }
}
