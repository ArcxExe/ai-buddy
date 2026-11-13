# AI-buddy — Frontend

React + TypeScript + Vite, React Router, TanStack Query, Zustand и Zod.
Пять маршрутов: `/`, `/chat`, `/onboarding`, `/knowledge`, `/requests`.
Серверные данные находятся в Query; Zustand содержит только состояние интерфейса.
Черновики сохраняются в sessionStorage отдельно для пользователя, роли и заявки.

## Структура

```text
src/
  api/          # HTTP-клиент, Zod-схемы и TanStack Query
  components/   # Оболочка, чат, адаптация и общие UI-компоненты
  pages/        # Главная, чат, база знаний и заявки
  lib/          # Черновики и форматирование
  store/        # Только локальное UI-состояние Zustand
  test/         # Unit/component-тесты и браузерный runner
  types/        # TypeScript-модели API
```

## Требования и установка зависимостей

Нужны Node.js 22+ и npm в `PATH`. Для локального Backend требуется Go 1.24+
(см. `backend/go.mod`). Проверенное окружение: Node.js 24.15.0, npm 11.12.1,
Go 1.27.4. Все команды ниже выполняются **из корня репозитория `ai-buddy`**
в Windows PowerShell, если явно не указано другое.

```powershell
git clone https://github.com/ArcxExe/ai-buddy.git
cd ai-buddy
npm.cmd --prefix frontend install
```

Для воспроизводимой установки по существующему lock-файлу вместо `install`
используйте `npm.cmd --prefix frontend ci`. Зависимости устанавливаются один раз
после клонирования и повторно при изменении `package-lock.json`.
В Linux/macOS используйте `npm` вместо `npm.cmd`. Команда `npm.cmd` позволяет
работать без изменения PowerShell ExecutionPolicy, блокирующей `npm.ps1`.

## Development: запуск для разработчика

В первом терминале запустите Backend (он нужен для всех сценариев с API):

```powershell
.\backend\start-local.cmd
```

Во втором терминале из корня:

```powershell
npm.cmd --prefix frontend run dev -- --host 127.0.0.1 --port 5174
```

Откройте http://127.0.0.1:5174. Проверка прокси:
http://127.0.0.1:5174/api/v1/health.

Это режим разработки: Vite подхватывает изменения исходников без ручной
пересборки. Доступны переключение демо-ролей, сброс демо и проверочный
RBAC-сценарий. Они меняют общее состояние Backend; используйте отдельный
демонстрационный экземпляр. Этот режим не предназначен для публикации
конечному пользователю.

Без аргументов `npm.cmd --prefix frontend run dev` слушает `127.0.0.1:5173`.
Явный `--port 5174` в инструкции переопределяет это значение. При занятом порте
dev-сервер завершится с ошибкой, а не выберет другой адрес автоматически.

## Production: сборка и локальный preview

```powershell
npm.cmd --prefix frontend run build
npm.cmd --prefix frontend run preview -- --host 127.0.0.1 --port 4174
```

Откройте http://127.0.0.1:4174. Сборка создаёт `frontend/dist`. Preview показывает
именно собранную production-версию: после изменения исходников снова выполните
`build`. Служебные переключение ролей, reset, RBAC-пресет и демо-баннер в ней
скрыты; профиль и права по-прежнему берутся из Backend. Это не заменяет
серверную авторизацию.

Backend должен оставаться запущенным. Локальный preview наследует `/api` proxy
из `server.proxy` в `vite.config.ts`, включая `API_PROXY_TARGET`. Проверка:
http://127.0.0.1:4174/api/v1/health.

`preview` — локальная проверка сборки, **не production-хостинг**. Он наследует
`strictPort=true` из конфигурации Vite: при занятом порте завершится с ошибкой.
Выберите другой порт явно (например, `--port 4175`) и проверяйте URL в терминале.

### Размещение собранного приложения

Раздавайте содержимое `frontend/dist` через веб-сервер с HTTPS. Он должен:

- Возвращать `index.html` для клиентских маршрутов `/chat`, `/onboarding`,
  `/knowledge`, `/requests`, включая прямое открытие и обновление страницы.
- Направлять `/api/v1/*` на доступный Go Backend, сохраняя путь запроса.
- Обрабатывать API отдельно от SPA fallback: ошибка API не должна возвращать HTML.

Frontend использует относительный `/api/v1`: отдельной переменной браузерного
API URL сейчас нет. `API_PROXY_TARGET` управляет dev/preview Vite и не встраивает
адрес Backend в `dist`. В production настраивается reverse proxy веб-сервера.
Существующий `frontend/Dockerfile` содержит Nginx с SPA fallback и прокси
`/api/` на `http://backend:8080/api/`; Docker-конфигурация не менялась.
GitHub Pages сам по себе не запускает Go и не предоставляет такой API proxy.
Публикация исходников в GitHub не означает развёртывание приложения.

## Подключение Backend

Проверка автономного Backend: http://127.0.0.1:18080/api/v1/health.
Существующий `backend/start-local.cmd` запускает memory/fallback без PostgreSQL,
Redis и ключа LLM. Переменные ограничены процессом скрипта и его дочерними
процессами (`setlocal`); корневой `.env` скрипт не читает. Порт можно передать
аргументом: `.\backend\start-local.cmd 18082`. Данные исчезают при остановке
этого экземпляра. Подробнее о Docker и Linux/macOS — в [корневом README](../README.md).

Для остановки каждого запущенного сервера нажмите Ctrl+C в его терминале;
если Windows спрашивает `Terminate batch job (Y/N)?`, ответьте `Y`.
Не останавливайте чужие процессы ради освобождения порта.

## Переменные окружения

По умолчанию никаких env-файлов для локального запуска не требуется. При
необходимости скопируйте **`frontend/.env.example`** в **`frontend/.env.local`**,
не перезаписывая существующий файл:

```powershell
if (!(Test-Path .\frontend\.env.local)) {
  Copy-Item .\frontend\.env.example .\frontend\.env.local
}
```

Настройки читаются Vite, а не браузерным приложением:

| Переменная | Значение по умолчанию | Назначение |
|---|---|---|
| `DEV_HOST` | `127.0.0.1` | Интерфейс dev-сервера |
| `DEV_PORT` | `5173` | Строго фиксированный порт |
| `API_PROXY_TARGET` | `http://127.0.0.1:18080` | Прокси `/api` для dev и preview |

Приоритет: переменные процесса выше env-файлов; CLI `--host`/`--port`
переопределяют соответствующие параметры сервера. Vite также читает `.env`,
`.env.[mode]` и `.env.[mode].local` внутри `frontend`; `dev` использует mode
`development`, `build` и `preview` — `production`. `.env.local` действует в обоих
режимах. После изменения env перезапустите Vite. Для обычного статического
хостинга эти переменные не заменяют конфигурацию reverse proxy.

Корневой `.env.example` относится к Backend/Docker, это другой файл. Копирование
его в `.env` само по себе не экспортирует переменные в PowerShell или `go run`.

Если порты заняты, используйте другие, не останавливая сторонние процессы:

```powershell
# Терминал 1
.\backend\start-local.cmd 18082

# Терминал 2, переменная только текущего терминала и его дочерних процессов
$env:API_PROXY_TARGET = 'http://127.0.0.1:18082'
npm.cmd --prefix frontend run dev -- --host 127.0.0.1 --port 5175
# Или, после build:
# npm.cmd --prefix frontend run preview -- --host 127.0.0.1 --port 4175 --strictPort
```

Владельцев портов можно посмотреть без завершения процессов:

```powershell
Get-NetTCPConnection -State Listen -LocalPort 5174,4174,18080 -ErrorAction SilentlyContinue |
  Select-Object LocalAddress,LocalPort,OwningProcess
```

Никогда не добавляйте API-ключи LLM и токены в `VITE_*`, исходники или `.env.example`.
Локальные env-файлы, зависимости, dist и отчёты тестов исключены из Git.

## Тестирование

```powershell
npm.cmd --prefix frontend run typecheck
npm.cmd --prefix frontend test
npm.cmd --prefix frontend run test:coverage
npm.cmd --prefix frontend run build
git diff --check
```

Отчёт покрытия: `frontend/coverage/index.html`. Не публикуйте сгенерированные
отчёты, `node_modules` и `dist` в коммите исходников.

## API и LLM

Клиент использует относительный `/api/v1`, валидирует ответы Zod и отправляет
сообщения только через `POST /api/v1/chat/message`. Подключение провайдера,
ключи и модель остаются ответственностью Backend; браузер не обращается к LLM
напрямую. Нет фиктивной авторизации или подставных успешных API-ответов.

Markdown, источники, история, карточки действий и черновики работают независимо
от поставщика LLM. Сейчас ответ приходит целиком. SSE/WebSocket для потоковой
генерации — отдельная будущая задача после согласования серверного контракта.

## Известные ограничения

Замороженный Backend содержит синтетические профили, базу знаний и локальные
корпоративные сервисы. Метки `Mock`/`Synthetic` в серверных данных сохраняются;
доступы, отгулы и встречи нельзя считать реальными корпоративными операциями.
Показатель экономии времени является оценкой. Удаление служебных кнопок не
устраняет известные серверные ограничения авторизации и изоляции пользователей.

## Браузерная проверка

`src/test/browser-check.mjs` использует встроенный WebSocket Node.js и Chrome CDP,
без дополнительных библиотек. Запускайте **только с новым изолированным Backend**:
сценарий создаёт тестовые заявки, меняет роли и сбрасывает состояние. В production
preview смена роли/reset выполняются тестовым драйвером через существующий API,
а не через скрытые пользовательские кнопки. Нужен отдельный Chrome с временным
профилем и включённым remote debugging, закрываемый после проверки.

```powershell
$env:AI_BUDDY_ISOLATED_TEST = '1'
$env:BASE_URL = 'http://127.0.0.1:4174'
$env:CDP_URL = 'http://127.0.0.1:19222'
# Runner выполняется из frontend, чтобы отчёт попал в frontend/coverage/browser
Push-Location frontend
try { node src/test/browser-check.mjs } finally { Pop-Location }
```

Unit/component-тесты и browser runner находятся в `src/test`. Браузерный отчёт
и скриншоты записываются в `coverage/browser` и не входят в исходники публикации.
Проверки не заменяют аудит Git-истории на секреты и проверку реальной корпоративной
авторизации перед эксплуатационным развёртыванием.
