# License Service API

Backend-сервис для управления и валидации программных лицензий. Построен с использованием Go, Gin, PostgreSQL, Redis и Asynq.

## Предварительные Требования

-   Go 1.21 или выше
-   `migrate` CLI (инструкции по установке: [golang-migrate/migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate))
-   Доступ к работающему PostgreSQL и Redis (или запуск через Docker Compose)

## Начало Работы

1.  **Настроить Переменные Окружения:**

    -   Скопируйте пример файла: `cp .env.example .env`
    -   Отредактируйте файл `.env`, указав ваши настройки для:
        -   `DATABASE_URL`: Строка подключения к PostgreSQL (например, `postgres://user:password@localhost:5432/licensing_db?sslmode=disable`)
        -   `REDIS_ADDR`: Адрес Redis (например, `localhost:6379`)
        -   `REDIS_PASSWORD`: Пароль Redis (если есть, иначе оставить пустым)
        -   `JWT_SECRET_KEY`: **Очень важный**, длинный и сложный секретный ключ для подписи JWT.
        -   `SERVER_PORT`: Порт, на котором будет работать API (например, `8080`).
        -   `LOG_LEVEL`: Уровень логирования (`debug`, `info`, `warn`, `error`).
        -   Другие опциональные параметры (таймауты, TTL токена и т.д.).

2.  **Применить Миграции Базы Данных:**

    -   Убедитесь, что база данных, указанная в `DATABASE_URL`, существует.
    -   Выполните миграции (переменная `DATABASE_URL` должна быть доступна в окружении):

    ```bash
    # Установка переменной (если не загружается из .env автоматически)
    # export $(grep -v '^#' .env | xargs)

    migrate -database "$DATABASE_URL" -path ./migrations up
    ```

3.  **(Опционально) Создать Первый API Ключ:**

    -   Если вы еще не создали ключ для агента, используйте скрипт (если он есть) или вставьте запись вручную в таблицу `api_keys`. Пример скрипта (`cmd/createapikey/main.go`) был показан ранее. Не забудьте сохранить полный ключ!

4.  **Запустить Приложение:**
    -   **В режиме разработки:**
    ```bash
    go run ./cmd/server/main.go
    ```
    -   **Или собрать и запустить бинарник:**
    ```bash
    go build -o license-service-api ./cmd/server/main.go
    ./license-service-api
    ```
    -   **Или через Docker (если настроен `Dockerfile`):**
    ```bash
    docker build -t license-service-api .
    docker run -p 8080:8080 --env-file .env license-service-api
    ```

Сервер API должен запуститься на порту, указанном в `SERVER_PORT` (по умолчанию 8080).

**Основные Эндпоинты:**

-   `/healthz`: Проверка состояния сервиса.
-   `/metrics`: Метрики Prometheus.
-   `/api/v1/auth/login` (`POST`): Аутентификация пользователя (логин/пароль), возвращает JWT.
-   `/api/v1/licenses` (`POST`, `GET`): Создание и получение списка лицензий (требует JWT).
-   `/api/v1/licenses/{id}` (`GET`, `PATCH`): Получение и обновление лицензии по ID (требует JWT).
-   `/api/v1/licenses/{id}/status` (`PATCH`): Изменение статуса лицензии (требует JWT).
-   `/api/v1/licenses/validate` (`POST`): Валидация лицензионного ключа (требует `X-API-Key`).
-   `/api/v1/dashboard/summary` (`GET`): Получение данных для дашборда (требует JWT).
