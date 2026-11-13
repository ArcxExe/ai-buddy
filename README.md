# AI-buddy: ИИ-наставник для стажёров

> **Хакатон Avito Start (Hack&Change 2026)**  
> **Команда:** 5 человек (2 AI-разработчика с Gemini & Codex + 3 участника продуктового/аналитического трека)  
> **Видеодемонстрация:** ссылка пока не добавлена (см. раздел в конце).

React-интерфейс с пятью разделами: главная, ИИ-помощник, адаптация, база знаний
и заявки. Подробные инструкции, структура Frontend, переменные окружения и
ограничения — в **[frontend/README.md](frontend/README.md)**.

Для Windows ниже приведены два разных режима: [Development](#windows-development)
на `127.0.0.1:5174` и [production preview](#production-preview) на `127.0.0.1:4174`.
Backend требуется в обоих. Публикация исходников на GitHub не запускает приложение.

---

## 📌 Оглавление

1. [О проекте и бизнес-эффект](#about)
2. [Архитектура и стек технологий](#stack)
3. [Быстрый запуск: Docker, Linux/macOS, Windows](#quickstart)
4. [Production-сборка и preview](#production-preview)
5. [Демонстрационные профили и сценарии проверки](#demo-scenarios)
6. [Спецификации SDD и MCP](#specifications)
7. [Тестирование и покрытие](#testing)

---

<a id="about"></a>

## 💡 О проекте и бизнес-эффект

По оценке команды Авито (слайд 5), занятость наставника (buddy) составляет **40–48 часов** (за 6 мес) и **55–65 часов** (за 9 мес). До 50% этого времени уходит на повторяющиеся бытовые и процессные вопросы.

**Целевые гипотезы**, а не измеренные результаты: снижение рутинной нагрузки на
buddy на **40–45%** (18–22 часа) и сокращение `Time-to-First-PR` с 12 до 7 рабочих дней.

**AI-buddy** в текущем прототипе:

- Проводит сценарии оформления учебных отгулов, запроса доступов в IDM и синхронизации чек-листа через локальные **Mock-сервисы**.
- Проверяет ролевую доступность документов и инструментов. Это демонстрационная RBAC-модель, а не полноценная корпоративная авторизация.
- **Не заменяет содержательную работу наставника:** при архитектурных вопросах автоматически формирует структурированную повестку для 15-минутного синка 1:1.

---

<a id="stack"></a>

## 🛠 Архитектура и стек технологий (Авито)

Технологии текущего прототипа:

| Слой | Технологии | Назначение |
| :--- | :--- | :--- |
| **Backend** | **Go 1.24+** (`net/http` pattern routing) | Модули `internal/rbac`, `internal/mcp`, `internal/rag`, `internal/ai`. Минимальная версия — в `backend/go.mod`. |
| **Frontend** | **React 19 + TypeScript + Vite** | Дизайн-система Avito Start, стилизация на **Tailwind CSS**, иконки **Lucide**. |
| **Стейт и данные** | **TanStack Query + Zustand + Zod** | Query — серверные данные; Zustand — локальный UI; Zod — валидация ответов API. |
| **Инструменты агента** | **MCP-модуль** | Локальные имитации `Avito Absence`, `Avito Access / IDM`, `Mentor 1:1 Sync` и `Clickstream`. |
| **Хранилища** | **Memory / PostgreSQL 16 + Redis 7** | Режим определяется конфигурацией Backend; автономный скрипт использует только память. |
| **LLM Runtime** | **OpenRouter API** + **Smart Deterministic Fallback** | При отсутствии ключа работает автономный fallback; провайдер подключается на Backend. |

---

<a id="quickstart"></a>

## 🚀 Быстрый запуск (Docker Compose за 1 команду)

### Способ 1. Запуск через Docker Compose (Рекомендуемый для жюри)

```bash
# 1. Склонируйте репозиторий
git clone https://github.com/ArcxExe/ai-buddy.git
cd ai-buddy

# 2. Создайте файл конфигурации (при необходимости укажите ключ OpenRouter)
test -f .env || cp .env.example .env

# 3. Запустите весь стек одной командой
docker compose up --build
```

После старта:

- **Веб-интерфейс прототипа:** [http://localhost:5173](http://localhost:5173) (production-сборка через Nginx).
- **REST API Backend:** [http://localhost:8080/api/v1/health](http://localhost:8080/api/v1/health)

> Если `OPENROUTER_API_KEY` пустая, используется **Smart Fallback Engine**.
> Текущий Compose передаёт DSN и Redis, но не `STORAGE_MODE`: по умолчанию Backend
> работает в memory-режиме, поэтому наличие PostgreSQL-контейнера не гарантирует
> сохранение состояния в PostgreSQL. Docker-конфигурация не изменялась.
> Пароль БД в примере — публичное демонстрационное значение, не production-секрет.
> В production UI скрыты демо-переключение роли и reset; сценарии жюри ниже
> выполняйте в Development на отдельном Backend.

### Способ 2. Локальный запуск разработчика (Linux/macOS)

```bash
# Терминал 1: Go Backend (Go 1.24+), автономное состояние
cd backend && BACKEND_PORT=18080 STORAGE_MODE=memory go run ./cmd/server

# Терминал 2: React Frontend (Node 22+)
cd frontend && npm install && npm run dev
```

<a id="windows-development"></a>

### Способ 3. Windows PowerShell: Development

Требуются **Go 1.24+** (см. `backend/go.mod`), **Node.js 22+** и npm в `PATH`.
Проверенное окружение: Go 1.27.4, Node.js 24.15.0, npm 11.12.1. После установки
инструментов откройте новый терминал; при необходимости перезапустите VS Code.

Откройте два терминала в корне репозитория `ai-buddy`:

```powershell
# Терминал 1: Backend; из backend можно выполнить .\start-local.cmd
.\backend\start-local.cmd

# Терминал 2: установка зависимостей и Frontend
npm.cmd --prefix frontend install
npm.cmd --prefix frontend run dev -- --host 127.0.0.1 --port 5174
```

Для установки строго по lock-файлу вместо `install` используйте
`npm.cmd --prefix frontend ci`. Установка требуется после клонирования и обновления
зависимостей. Development автоматически отображает изменения исходников без
ручного build. В нём доступны демо-роли и reset; не публикуйте dev-сервер для
конечных пользователей.

Из каталога `frontend` достаточно `npm.cmd run dev` (порт по умолчанию — 5173).
Используются `.cmd`, поскольку
PowerShell с политикой `Restricted` может блокировать `npm.ps1` и `.ps1`-скрипты.
Изменять ExecutionPolicy или системные настройки Windows не требуется.

- Frontend по команде выше: **http://127.0.0.1:5174/**.
- Проверка Backend: **http://127.0.0.1:18080/api/v1/health**.
- Проверка API через Vite: **http://127.0.0.1:5174/api/v1/health**.
- Клиент обращается к относительному `/api/v1`; Vite направляет `/api` на Backend.

`start-local.cmd` использует `setlocal`: переменные задаются только для запускаемого
процесса и его дочерних процессов. Включены `STORAGE_MODE=memory` и автономный
`smart_fallback`; PostgreSQL, Redis и OpenRouter не подключаются, ключ не нужен.
Корневой `.env` этот скрипт не загружает. Демо-данные исчезают после остановки Backend.

Параметры Vite по умолчанию сохранены в `frontend/vite.config.ts`: IPv4-адрес
`127.0.0.1`, порт `5173`, `strictPort=true`, прокси `http://127.0.0.1:18080`.
Для другого окружения можно один раз скопировать пример и изменить значения:

```powershell
if (!(Test-Path .\frontend\.env.local)) {
  Copy-Item .\frontend\.env.example .\frontend\.env.local
}
```

`DEV_HOST`, `DEV_PORT` и `API_PROXY_TARGET` читаются из `frontend/.env.local` или
окружения процесса. Файл `.env.local` игнорируется Git и сохраняется между
перезапусками VS Code; после изменения настроек перезапустите Vite. Для запуска
Backend на другом порту используйте, например, `.\backend\start-local.cmd 18081`
и укажите `API_PROXY_TARGET=http://127.0.0.1:18081` в `frontend/.env.local`.
Настройки касаются dev-сервера Vite: Docker по-прежнему использует Nginx и
`backend:8080`, Docker-конфигурация не меняется.

Для остановки нажмите **Ctrl+C в каждом терминале**. Если Windows спросит
`Terminate batch job (Y/N)?`, ответьте `Y`. Backend должен вывести
`Shutting down AI-buddy Backend cleanly...`. Не закрывайте терминал до завершения.

Если выбранный dev-порт занят, Vite завершится с ошибкой вместо смены порта.
Посмотреть владельца порта можно командой:

```powershell
Get-NetTCPConnection -State Listen -LocalPort 5174,4174,18080 -ErrorAction SilentlyContinue |
  Select-Object LocalAddress,LocalPort,OwningProcess
```

Остановите свой предыдущий экземпляр либо выберите другие порты через настройки
выше. Сторонний Apache на 8080 останавливать не нужно. В некоторых средах запуска
агента esbuild может получить `Access is denied` при чтении родительских каталогов;
это ограничение sandbox. Для обычного локального запуска используйте терминал VS Code.

---

<a id="production-preview"></a>

## Production-сборка и локальный preview

Команды из корня репозитория (Backend оставьте запущенным):

```powershell
npm.cmd --prefix frontend run build
npm.cmd --prefix frontend run preview -- --host 127.0.0.1 --port 4174
```

Откройте **http://127.0.0.1:4174**; проверка API —
**http://127.0.0.1:4174/api/v1/health**. Сборка создаётся в `frontend/dist`.
Preview показывает эту production-сборку, где служебные демо-инструменты скрыты.
После изменения исходников повторите `build`. Для остановки — Ctrl+C.
Если 4174 занят, preview завершится с ошибкой: он наследует `strictPort=true`
из конфигурации Vite. Выберите свободный порт явно, например `--port 4175`.

Preview не является production-хостингом. Для реального размещения нужен веб-сервер
с HTTPS, SPA fallback на `index.html` и reverse proxy `/api/v1` на Go Backend.
`API_PROXY_TARGET` используется только Vite dev/preview и не меняет API URL внутри
`dist`. Подробности — [подключение и окружение Frontend](frontend/README.md).
GitHub Pages не предоставляет Go Backend или этот API proxy.

<a id="demo-scenarios"></a>

## 👥 Демонстрационные профили и сценарии проверки жюри

В **Development** в шапке доступен демо-переключатель ролей. Это синтетические
профили, не учётные записи с паролями. В production показывается профиль из API:

| Роль | Тестовый сотрудник | Описание и доступные действия |
| :--- | :--- | :--- |
| **Стажёр (Intern)** | **Алексей Смирнов** (Backend Go Intern, Авито Доставка) | Доступен онбординг, чек-лист, чат с AI-buddy, создание заявок на отгул и доступы. Закрыты: прод-БД, зарплаты, согласование заявок. |
| **Наставник (Buddy)** | **Мария Козлова** (Senior Go Developer, Авито Доставка) | Согласование/отклонение заявок, просмотр повесток 1:1 и расчётных показателей. |
| **Руководитель (Manager)**| **Дмитрий Волков** (Engineering Manager, Авито Доставка) | Согласование заявок и доступные этой роли материалы базы знаний. |

### Порядок проверки 3 сценариев из кейса:

1. **Сценарий 1 («Доступы в IDM и окружение»):**
   - Нажмите быстрый пресет: *«🚀 Сценарий 1: Доступы в IDM»*.
   - AI-buddy объяснит регламент по `AvitoTech Playbook`, вызовет MCP-инструмент `request_service_access`, создаст одобренную заявку в **Avito Access / IDM** и автоматически отметит шаг чек-листа.
   - **Проверка RBAC (Негативный кейс):** Нажмите пресет *«🔒 Проверка RBAC: Прод-БД»*. AI-buddy и модуль `internal/rbac` заблокируют запрос к `prod-billing-db` с наглядным предупреждением безопасности.
2. **Сценарий 2 («Оформление отгула на экзамен» — кейс со слайда 4):**
   - Нажмите пресет: *«📝 Сценарий 2: Отгул на экзамен»*.
   - AI-buddy сформирует черновик заявки (**Human-in-the-Loop Action Card**).
   - Нажмите в карточке **«Подтвердить и отправить»**.
   - Переключите роль в шапке на **«Наставник (Мария)»** — заявка появится во входящих с кнопками *«Согласовать»* и *«Отклонить»*.
3. **Сценарий 3 («Первая задача и повестка к 1:1 с наставником» — слайд 5):**
   - Нажмите пресет: *«🤝 Сценарий 3: Повестка к 1:1»*.
   - AI-buddy вызовет MCP-инструмент `prepare_mentor_1on1_summary`, зафиксирует выполненную работу и выделит вопросы по кэшированию в Redis. Экономия времени в метриках — расчётная оценка.

---

<a id="specifications"></a>

## 📄 Спецификации SDD и MCP (Обязательное требование слайдов 8 и 11)

- **`specs/agent-sdd.yaml`** — формальная SDD-спецификация поведения агента (OpenSpec / spec-kit), системные промпты, guardrails и детерминированные графы.
- **`specs/mcp-tools.json`** — JSON Schema инструментов (`search_knowledge_base`, `get_onboarding_status`, `complete_checklist_item`, `request_service_access`, `create_absence_request`, `prepare_mentor_1on1_summary`, `review_corporate_request`).
- **`specs/openapi.yaml`** — проектная спецификация REST API; содержит расхождения с реализацией. Для текущей интеграции источник истины — `backend/internal/http/handlers.go` и `backend/internal/domain/models.go`; Frontend проверяет фактические ответы Zod-схемами.
- **`data/knowledge/avito_playbook_seed.json`** — синтетическая база знаний на базе открытого `AvitoTech Playbook` с разметкой прав доступа (`intern`, `mentor_only`, `manager_only`).

---

<a id="testing"></a>

## 🧪 Тестирование и воспроизводимый отчёт покрытия (>80%)

По критериям кейса (слайд 12, п. 07–08), покрытие прикладного кода автоматизированными тестами должно **превышать 80%**.

### Единая команда проверки:
```bash
make coverage
```

### Проверка Frontend из корня (Windows):

```powershell
npm.cmd --prefix frontend run typecheck
npm.cmd --prefix frontend test
npm.cmd --prefix frontend run test:coverage
npm.cmd --prefix frontend run build
git diff --check
```

Покрытие смотрите в фактическом выводе команды и `frontend/coverage/index.html`.
`make coverage` предназначен для Unix-подобной оболочки и создаёт сводные отчёты
в `coverage/`; он не заменяет отдельный успешный запуск тестов (его рецепт
подавляет ошибку Frontend-команды). Не используйте старые проценты как результат
нового запуска.

Запуск тестов по отдельности:
```bash
# Только Go тесты
make test-backend

# Только Frontend тесты
make test-frontend
```

---

## 🎥 Видеодемонстрация
Видеодемонстрация (2.5 минуты, путь от запроса стажёра до согласования наставником и срабатывания RBAC):
- Ссылка: `[Вставить ссылку перед отправкой на платформу]` (согласно слайду 8).

## Известные ограничения

Backend остаётся хакатонным прототипом: общее демонстрационное состояние,
синтетические данные и Mock-корпоративные действия; полноценной корпоративной
авторизации и изоляции сотрудников нет. Экономия времени — оценка. Ответ LLM
приходит целиком, streaming не реализован. Перед эксплуатационным размещением
нужны отдельные серверные доработки и аудит безопасности. Детали поведения UI
и локальной проверки — в [frontend/README.md](frontend/README.md).

---
*Сделано с ❤️ для хакатона Avito Start (Hack&Change 2026).*
