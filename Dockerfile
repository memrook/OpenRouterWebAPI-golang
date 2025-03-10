# Используем двухэтапную сборку для минимизации размера финального образа

# Этап сборки
FROM golang:1.21-alpine AS builder

# Установка рабочей директории внутри контейнера
WORKDIR /app

# Копирование и скачивание зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка приложения
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o openrouter-web-api .

# Этап создания финального образа
FROM alpine:latest

# Установка необходимых зависимостей
RUN apk --no-cache add ca-certificates tzdata && \
    mkdir -p /app/templates

# Установка рабочей директории
WORKDIR /app

# Копирование скомпилированного бинарного файла из этапа сборки
COPY --from=builder /app/openrouter-web-api .
COPY --from=builder /app/templates/ ./templates/

# Установка переменных окружения
ENV PORT=8080
# ENV OPENROUTER_API_KEY="Ваш_API_ключ"
# Не устанавливаем API ключ в образе из соображений безопасности
# Его нужно передавать при запуске контейнера

# Открываем порт, который будет прослушивать приложение
EXPOSE 8080

# Запуск приложения
CMD ["./openrouter-web-api"] 