package main

import (
	"bufio"
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
	"sync"
	"time"
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
	Stream      bool    `json:"stream"`
}

type DeepseekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Структура для SSE
type StreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// ConversationHistory Структура для истории переписки
type ConversationHistory struct {
	sync.Mutex
	Messages []Message
}

// Глобальная переменная для хранения истории
var conversationHistory = ConversationHistory{
	Messages: []Message{},
}

// Модель AI
const model = "google/gemini-2.0-flash-lite-preview-02-05:free"

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")
}

func main() {
	// Проверяем наличие API ключа
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENROUTER_API_KEY не настроен. Установите переменную окружения OPENROUTER_API_KEY.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/chat", handleChat)
	http.HandleFunc("/query", handleChat)
	http.HandleFunc("/stream", handleStream)
	http.HandleFunc("/reset", handleReset)
	http.HandleFunc("/api-status", handleAPIStatus)
	http.Handle("/templates/", http.StripPrefix("/templates/", http.FileServer(http.Dir("templates"))))

	log.Println("Сервер запущен на http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))

	// Передаем историю сообщений в шаблон
	conversationHistory.Lock()
	data := map[string]interface{}{
		"Messages": conversationHistory.Messages,
	}
	conversationHistory.Unlock()

	tmpl.Execute(w, data)
}

// Обработчик сброса истории
func handleReset(w http.ResponseWriter, r *http.Request) {
	conversationHistory.Lock()
	conversationHistory.Messages = []Message{}
	conversationHistory.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("История беседы сброшена"))
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

// Обработчик потокового режима
func handleStream(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)

	if r.Method == http.MethodOptions {
		return
	}

	// Поддерживаем как GET, так и POST запросы для SSE
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Настраиваем заголовки для SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var userMessage string
	if r.Method == http.MethodGet {
		// Для EventSource используется GET запрос с параметрами в URL
		userMessage = r.URL.Query().Get("message")
	} else {
		// Для обычных POST запросов используем FormValue
		userMessage = r.FormValue("message")
	}

	if userMessage == "" {
		http.Error(w, "Сообщение не может быть пустым", http.StatusBadRequest)
		return
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		http.Error(w, "API ключ не настроен", http.StatusInternalServerError)
		return
	}

	// Добавляем сообщение пользователя в историю
	conversationHistory.Lock()
	conversationHistory.Messages = append(conversationHistory.Messages, Message{
		Role:    "user",
		Content: userMessage,
	})

	// Создаем копию сообщений для запроса
	messagesCopy := make([]Message, len(conversationHistory.Messages))
	copy(messagesCopy, conversationHistory.Messages)
	conversationHistory.Unlock()

	reqBody := DeepseekRequest{
		Model:       model,
		Messages:    messagesCopy,
		Temperature: 1,
		Stream:      true, // Включаем стриминг
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

	log.Printf("Отправка запроса на API OpenRouter с параметрами: %+v", reqBody)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Ошибка при отправке запроса: %v", err)
		http.Error(w, "Ошибка при отправке запроса: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("API вернул ошибку: %s - %s", resp.Status, string(body))
		http.Error(w, fmt.Sprintf("API вернул ошибку: %s - %s", resp.Status, string(body)), resp.StatusCode)
		return
	}

	log.Printf("Успешный ответ от API, начинаю потоковую передачу клиенту")

	// Создаем флашер для принудительной отправки данных клиенту
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming не поддерживается", http.StatusInternalServerError)
		return
	}

	// Чтение потока с ответом
	reader := bufio.NewReader(resp.Body)

	// Буфер для накопления ответа полностью
	fullResponseContent := ""

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Ошибка чтения потока: %v", err)
			break
		}

		// Пропускаем пустые строки
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Обрабатываем данные в формате SSE
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Проверка окончания потока
			if data == "[DONE]" {
				// Отправляем финальный маркер клиенту
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				break
			}

			// Парсим JSON с частью ответа
			var streamResp StreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				log.Printf("Ошибка парсинга JSON: %v", err)
				continue
			}

			// Получаем новую часть контента
			if len(streamResp.Choices) > 0 {
				content := streamResp.Choices[0].Delta.Content
				if content != "" {
					// Накапливаем полный ответ
					fullResponseContent += content

					// Отправляем часть ответа клиенту в правильном формате SSE
					// Отправляем исходный JSON для совместимости с клиентским кодом
					_, err := fmt.Fprintf(w, "data: %s\n\n", data)
					if err != nil {
						log.Printf("Ошибка при отправке данных клиенту: %v", err)
						return
					}
					flusher.Flush()
				}
			}
		}
	}

	// Добавляем ответ ассистента в историю
	if fullResponseContent != "" {
		conversationHistory.Lock()
		conversationHistory.Messages = append(conversationHistory.Messages, Message{
			Role:    "assistant",
			Content: fullResponseContent,
		})
		conversationHistory.Unlock()
	}
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

	// Добавляем сообщение пользователя в историю
	conversationHistory.Lock()
	conversationHistory.Messages = append(conversationHistory.Messages, Message{
		Role:    "user",
		Content: userMessage,
	})

	// Создаем копию сообщений для запроса
	messagesCopy := make([]Message, len(conversationHistory.Messages))
	copy(messagesCopy, conversationHistory.Messages)
	conversationHistory.Unlock()

	reqBody := DeepseekRequest{
		Model:       model,
		Messages:    messagesCopy,
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

	log.Printf("Отправка запроса на API OpenRouter с параметрами: %+v", reqBody)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Ошибка при отправке запроса: %v", err)
		http.Error(w, "Ошибка при отправке запроса: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("API вернул ошибку: %s - %s", resp.Status, string(body))
		http.Error(w, fmt.Sprintf("API вернул ошибку: %s - %s", resp.Status, string(body)), resp.StatusCode)
		return
	}

	log.Printf("Успешный ответ от API, начинаю потоковую передачу клиенту")

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

		// Добавляем ответ ассистента в историю
		conversationHistory.Lock()
		conversationHistory.Messages = append(conversationHistory.Messages, Message{
			Role:    "assistant",
			Content: deepseekResp.Choices[0].Message.Content,
		})
		conversationHistory.Unlock()

		w.Write([]byte(formattedText))
	} else {
		http.Error(w, "Пустой ответ от API", http.StatusInternalServerError)
	}
}

// Обработчик для проверки статуса API
func handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)

	w.Header().Set("Content-Type", "application/json")

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "API ключ не настроен",
		})
		return
	}

	// Создаем простой запрос к API для проверки статуса
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Ошибка при создании запроса: " + err.Error(),
		})
		return
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Ошибка при отправке запроса: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		w.WriteHeader(resp.StatusCode)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": fmt.Sprintf("API вернул ошибку: %s - %s", resp.Status, string(body)),
		})
		return
	}

	// Если всё хорошо, возвращаем успешный статус
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "API работает нормально",
	})
}
