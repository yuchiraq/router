# Router — reverse proxy + admin panel + security telemetry

`router` — это единый Go‑сервис, который одновременно:

- принимает внешний HTTP/HTTPS трафик;
- маршрутизирует запросы на внутренние сервисы по `Host`;
- предоставляет административную панель для управления правилами;
- собирает системную и сетевую статистику;
- показывает поток логов в реальном времени;
- отслеживает подозрительные IP и позволяет банить их из UI с сохранением в JSON.

---

## 1) Архитектура и дизайн сервиса

Сервис построен как **монолит из независимых внутренних модулей** (`internal/*`), где каждый модуль отвечает за свою область:

- `internal/proxy` — data-plane (боевой трафик, reverse proxy).
- `internal/panel` — control-plane (админ UI + API статистики + действия оператора).
- `internal/stats` — сбор/агрегация метрик (CPU, RAM, SSH, запросы, страны, диски).
- `internal/storage` — персистентные JSON-хранилища (`rules.json`, `ip_reputation.json`).
- `internal/logstream` — fan-out логов в WebSocket клиентов.

### Ключевая идея дизайна

Сервис разделяет:

- **управление** (admin panel `:8162`);
- **прокси-трафик** (`:80/:443`);
- **локальные state-файлы** (JSON) для простого и предсказуемого восстановления после рестарта.

Это делает проект простым в эксплуатации без внешней БД: достаточно бинарника + каталога данных.

---

## 2) Поток запроса (request lifecycle)

1. Клиент приходит на `:443` (или `:80` для ACME/служебного HTTP).
2. `proxy` проверяет IP в репутационном хранилище:
   - если IP в бане -> `403 Forbidden`.
3. Проверяется глобальный maintenance mode.
4. По `Host` ищется правило маршрутизации.
   - если правила нет: `404`, IP помечается как suspicious (`unknown host`).
5. Дополнительно детектируются probe‑пути (`.env`, `wp-admin`, `phpmyadmin` и т.п.)
   - IP помечается suspicious (`suspicious path probe`).
6. Запрос проксируется на target сервиса.
7. Статистика запросов/стран обновляется в `stats`.

---

## 3) Дизайн административной панели

Панель построена как **single-page-like dashboard** на server-rendered HTML + JS polling:

- HTML: `internal/panel/static/stats.html`
- CSS: `internal/panel/static/styles.css`
- Графики: локальный `chart.js`

### Визуальный стиль

- glassmorphism-поверхности (`backdrop-filter`, полупрозрачные карточки);
- адаптивная сетка и карточки для телеметрии;
- поддержка `prefers-color-scheme: dark`;
- фиксированная верхняя навигация;
- фоновый градиент, закрепленный через `background-attachment: fixed`.

### Исправление фона при скролле

Чтобы фон больше **не обрывался при прокрутке**, в стилях применены:

- `min-height: 100%` для `html/body`;
- `min-height: 100vh` для `body`;
- фиксированный фон `background-attachment: fixed` + `background-repeat: no-repeat`.

---

## 4) Безопасность и anti-abuse дизайн

### 4.1 BasicAuth панели

Если заданы `ADMIN_USER` и `ADMIN_PASS`, панель и action endpoints защищены BasicAuth.

### 4.2 Репутация IP (JSON persistence)

Файл `ip_reputation.json` хранит:

- IP;
- reason;
- count;
- firstSeen / lastSeen;
- banned / bannedAt.

Это дает:

- долгоживущую память о suspicious активностях;
- возможность ручного контроля оператором;
- блокировку на уровне прокси до маршрутизации.

### 4.3 UI-операции безопасности

На странице статистики есть блок **Suspicious IPs**:

- список подозрительных адресов;
- причина и количество срабатываний;
- кнопка `Ban` (POST `/stats/ban`).

После бана запись становится помеченной, а трафик с IP режется `403`.

---

## 5) Хранилища данных

### `rules.json`

Используется для правил маршрутизации и глобального maintenance режима.

Пример:

```json
{
  "rules": {
    "example.com": {
      "target": "localhost:3000",
      "maintenance": false
    }
  },
  "maintenanceMode": false
}
```

### `ip_reputation.json`

Используется для security telemetry и банов.

Пример:

```json
{
  "entries": {
    "203.0.113.7": {
      "ip": "203.0.113.7",
      "reason": "suspicious path probe",
      "count": 5,
      "firstSeen": "2026-01-10T10:22:33Z",
      "lastSeen": "2026-01-10T11:47:02Z",
      "banned": true,
      "bannedAt": "2026-01-10T11:48:00Z"
    }
  }
}
```

---

## 6) Наблюдаемость (observability)

### Логи

- стандартный output + WebSocket broadcast;
- UI может читать поток в реальном времени через `/ws/logs`.

### Метрики

`stats` поддерживает:

- память;
- CPU;
- запросы по хостам;
- запросы по странам;
- активные SSH подключения;
- диски;
- suspicious IP список.

Фронтенд обновляется polling-ом (`/stats/data`).

---

## 7) Порты и рантайм

- `:80` — HTTP (ACME handler);
- `:443` — HTTPS reverse proxy;
- `:8162` — admin panel.

TLS — `autocert` (`golang.org/x/crypto/acme/autocert`).

---

## 8) Структура проекта

```text
.
├── main.go
├── internal/
│   ├── clog/
│   ├── config/
│   ├── logstream/
│   ├── panel/
│   │   ├── handlers.go
│   │   ├── templates/
│   │   └── static/
│   ├── proxy/
│   ├── stats/
│   └── storage/
│       ├── rules.go
│       ├── storage.go
│       └── ip_reputation.go
├── rules.json
├── ip_reputation.json
└── README.md
```

---

## 9) Быстрый старт

```bash
export ADMIN_USER=admin
export ADMIN_PASS=secret

go run main.go
```

После запуска:

- панель: `http://<host>:8162`
- прокси HTTPS: `https://<ваши-домены-из-rules.json>`

---

## 10) Дальнейшее развитие дизайна

Рекомендуемые next steps:

- CIDR-сети и маски банов (не только single IP);
- rate limiting / fail2ban-подобные авто-баны;
- audit trail действий оператора (кто и когда забанил);
- экспорт метрик в Prometheus;
- разделение operator/read-only ролей в панели.
