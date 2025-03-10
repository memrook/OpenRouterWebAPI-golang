package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepseekRequest struct {
	Model          string    `json:"model"`
	Messages       []Message `json:"messages"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
	Temperature float64 `json:"temperature"`
}

type DeepseekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")
}

func main() {
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/chat", handleChat)

	log.Println("Сервер запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	tmpl.Execute(w, nil)
}

func formatLatex(text string) string {
	// Сохраняем блоки кода
	codeBlocks := make([]string, 0)
	text = regexp.MustCompile("```[\\w]*[\\s\\S]*?```").ReplaceAllStringFunc(text, func(match string) string {
		codeBlocks = append(codeBlocks, match)
		return fmt.Sprintf("___CODE_BLOCK_%d___", len(codeBlocks)-1)
	})

	// Заменяем \boxed на div с классом
	text = regexp.MustCompile(`\\boxed\{([^}]*)\}`).ReplaceAllString(text, `<div class="latex-block">$1</div>`)

	// Обрабатываем \text команды внутри математического режима
	text = regexp.MustCompile(`\\text\{([^}]*)\}`).ReplaceAllString(text, `\text{$1}`)

	// Заменяем \linebreak на HTML перенос строки
	text = strings.ReplaceAll(text, "\\linebreak", "\n")

	// Заменяем двойные обратные слеши на одинарные
	text = strings.ReplaceAll(text, "\\\\", "\\")

	// Обрабатываем переносы строк
	text = strings.ReplaceAll(text, "\\n", "\n")
	text = strings.ReplaceAll(text, "\\r", "")

	// Восстанавливаем блоки кода с определением языка
	for i, block := range codeBlocks {
		lang := ""
		code := block[3 : len(block)-3]

		// Извлекаем язык программирования из первой строки
		if strings.Contains(code, "\n") {
			parts := strings.SplitN(code, "\n", 2)
			if len(parts) == 2 && len(parts[0]) > 0 {
				lang = strings.TrimSpace(parts[0])
				code = strings.TrimSpace(parts[1])
			}
		}

		if lang != "" {
			text = strings.ReplaceAll(text, fmt.Sprintf("___CODE_BLOCK_%d___", i),
				fmt.Sprintf("\n```%s\n%s\n```\n", lang, code))
		} else {
			text = strings.ReplaceAll(text, fmt.Sprintf("___CODE_BLOCK_%d___", i),
				fmt.Sprintf("\n```\n%s\n```\n", code))
		}
	}

	return text
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)

	if r.Method == http.MethodOptions {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	userMessage := r.FormValue("message")
	if userMessage == "" {
		http.Error(w, "Сообщение не может быть пустым", http.StatusBadRequest)
		return
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		http.Error(w, "API ключ не настроен", http.StatusInternalServerError)
		return
	}

	reqBody := DeepseekRequest{
		Model: "google/gemini-2.0-pro-exp-02-05:free",
		//Model: "deepseek/deepseek-r1-zero:free",
		Messages: []Message{
			{
				Role:    "user",
				Content: userMessage,
			},
		},
		Temperature: 1,
	}
	reqBody.ResponseFormat.Type = "text"

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		http.Error(w, "Ошибка при формировании запроса", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, "Ошибка при создании запроса", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "http://localhost:8080")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Ошибка при отправке запроса", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var deepseekResp DeepseekResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Ошибка при чтении ответа", http.StatusInternalServerError)
		return
	}

	err = json.Unmarshal(body, &deepseekResp)
	if err != nil {
		http.Error(w, "Ошибка при разборе ответа: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(deepseekResp.Choices) > 0 {
		formattedText := formatLatex(deepseekResp.Choices[0].Message.Content)
		w.Write([]byte(formattedText))
	} else {
		http.Error(w, "Пустой ответ от API", http.StatusInternalServerError)
	}
}
