# OpenRouter Web API

Интерфейс для взаимодействия с различными моделями ИИ через единый веб-интерфейс. Приложение позволяет переключаться между различными моделями и поддерживает как обычный, так и потоковый (streaming) режим генерации ответов.

## Возможности

- 💬 Чат-интерфейс с форматированием Markdown
- 🧮 Поддержка рендеринга формул LaTeX 
- 🌈 Подсветка синтаксиса для блоков кода
- 🔄 Переключение между разными моделями ИИ
- ⚡ Потоковый режим (streaming) для мгновенного отображения ответов
- 📋 Копирование ответов в буфер обмена
- 📝 Сохранение контекста беседы
- ⏱️ Отображение времени генерации ответа
- ❌ Возможность отмены генерации в потоковом режиме

<img width="960" alt="изображение" src="https://github.com/user-attachments/assets/70b7f8a7-a3de-49c2-a27a-733029791c2c" />

## Поддерживаемые модели

- Google: Gemini Flash Lite 2.0 Preview
- DeepSeek: DeepSeek R1 Zero
- Qwen: QwQ 32B

## Требования

- Go 1.21 или выше
- API ключ OpenRouter

## Установка

### Локальный запуск

1. Клонируйте репозиторий:
```bash
git clone https://github.com/memrook/OpenRouterWebAPI-golang.git
cd OpenRouterWebAPI-golang
```

2. Установите переменную окружения с вашим API ключом OpenRouter:
```bash
export OPENROUTER_API_KEY="your_api_key_here"
```

3. Запустите приложение:
```bash
go run main.go
```

4. Откройте браузер и перейдите по адресу `http://localhost:8080`

### Запуск с использованием Docker

1. Клонируйте репозиторий:
```bash
git clone https://github.com/memrook/OpenRouterWebAPI-golang.git
cd OpenRouterWebAPI-golang
```

2. Соберите Docker-образ:
```bash
docker build -t openrouter-web-api .
```

3. Запустите контейнер с указанием вашего API ключа:
```bash
docker run -p 8080:8080 -e OPENROUTER_API_KEY="your_api_key_here" openrouter-web-api
```

4. Откройте браузер и перейдите по адресу `http://localhost:8080`

#### Дополнительные опции Docker

- Изменение порта:
```bash
docker run -p 3000:8080 -e OPENROUTER_API_KEY="your_api_key_here" openrouter-web-api
```

- Запуск контейнера в фоновом режиме:
```bash
docker run -d -p 8080:8080 -e OPENROUTER_API_KEY="your_api_key_here" openrouter-web-api
```

- Использование Docker Compose (создайте файл docker-compose.yml):
```yaml
version: '3'
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - OPENROUTER_API_KEY=your_api_key_here
    restart: unless-stopped
```

И запустите:
```bash
docker-compose up -d
```

## Конфигурация

Вы можете изменить порт, на котором запускается приложение, установив переменную окружения `PORT`:
```bash
export PORT="3000"
```

## Использование

### Начало работы
1. При запуске приложения вы увидите чистый интерфейс с полем ввода и списком доступных моделей.
2. Выберите желаемую модель ИИ из выпадающего списка.
3. Введите ваше сообщение и нажмите кнопку "Отправить" или используйте сочетание клавиш Cmd+Enter / Ctrl+Enter.

### Режимы работы
- **Потоковый режим (Стриминг)**: По умолчанию включен. Ответы отображаются по мере их генерации. Можно отменить запрос в процессе.
- **Обычный режим**: Ответ отображается только после полного завершения генерации.

### История беседы
- Все сообщения сохраняются в истории беседы.
- Вы можете сбросить историю, нажав кнопку "Сбросить историю".

## Технические особенности

- Приложение написано на Go с использованием стандартной библиотеки `net/http`.
- Для форматирования на стороне клиента используются:
  - Marked.js для рендеринга Markdown
  - highlight.js для подсветки синтаксиса в блоках кода
  - MathJax для отображения формул LaTeX
- Для потокового режима используется технология Server-Sent Events (SSE).
- API запросы выполняются через OpenRouter, что позволяет использовать различные модели ИИ.

## Структура проекта

```
OpenRouterWebAPI-golang/
├── main.go           # Основной код сервера
├── templates/        # HTML шаблоны и статические файлы
│   └── index.html    # Основной шаблон интерфейса
├── Dockerfile        # Инструкции для сборки Docker-образа
└── README.md         # Документация проекта
```

## Развертывание на удаленном сервере

### Используя Docker

1. Загрузите свой Docker-образ в реестр (Docker Hub, GitHub Container Registry и т.д.):
```bash
docker tag openrouter-web-api yourusername/openrouter-web-api:latest
docker push yourusername/openrouter-web-api:latest
```

2. На удаленном сервере выполните:
```bash
docker pull yourusername/openrouter-web-api:latest
docker run -d -p 80:8080 -e OPENROUTER_API_KEY="your_api_key_here" --restart unless-stopped yourusername/openrouter-web-api:latest
```

### Используя systemd (без Docker)

1. Скопируйте файлы на сервер
2. Создайте systemd сервис `/etc/systemd/system/openrouter-web-api.service`:
```
[Unit]
Description=OpenRouter Web API
After=network.target

[Service]
User=yourusername
WorkingDirectory=/path/to/OpenRouterWebAPI-golang
ExecStart=/path/to/OpenRouterWebAPI-golang/openrouter-web-api
Environment="OPENROUTER_API_KEY=your_api_key_here"
Environment="PORT=80"
Restart=always

[Install]
WantedBy=multi-user.target
```

3. Включите и запустите сервис:
```bash
sudo systemctl enable openrouter-web-api
sudo systemctl start openrouter-web-api
```

## Лицензия

MIT

## Авторы

- [@memrook](https://github.com/memrook) 
