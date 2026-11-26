# Design: Simplify WorkAiStrator Orchestrator

## Context

Ядро WorkAiStrator размазано по ~25 пакетам. Границы размыты: гейты живут в 6
пакетах, этапы — в 4, машина состояний — в 4. Зависимости идут в обе стороны
(web → pipeline → runtime → web). При этом семантика продукта стабильна:
графовый workflow, переходы по вердиктам, гейты, доказательства, delivery по
подтверждённому плану. Рефакторим структуру, не трогая поведение.

## Goals / Non-Goals

**Goals:**
- Три слоя с однонаправленными зависимостями: `engine/` → `runtime/`, `store/`.
  Engine импортирует только стандартную библиотеку и свои подпакеты.
- Единые интерфейсы `Executor`, `GateRunner`, `EventSink` как единственные
  точки связи engine с внешним миром.
- ~8 пакетов вместо ~25; каждый пакет — одна ответственность (SRP), расширение
  новыми гейтами/исполнителями без правки ядра (OCP).
- Онбординг: `workaistrator init` + встроенный demo + 3 страницы docs.

**Non-Goals:**
- Изменение семантики графа, переходов, гейтов, доказательств, delivery.
- Изменение YAML-формата workflow и CLI-команд.
- Удаление существующих адаптеров (Jira, OIDC, TeamCity, MCP) — только
  изоляция за интерфейсами.
- Big-bang переписывание: работаем инкрементальными PR с зелёными тестами.

## Decisions

### 1. Три слоя вместо плоского pkg/

```
cmd/workaistrator     — композиция (wire), единственное место, где engine
                        соединяется с конкретными runtime/store
pkg/engine/           — граф, машина состояний, переходы, планировщик
pkg/engine/gate/      — определение, запуск и решение гейтов
pkg/engine/stage/     — исполнение этапа и проекция статусов
pkg/runtime/          — адаптеры: LLM (OpenCode/Codex/Claude), Docker, git
pkg/store/            — SQLite, events, manifests
pkg/server/           — HTTP/MCP слой над engine
```

Направление зависимостей: `cmd → engine → (интерфейсы)`, `runtime` и `store`
реализуют интерфейсы engine, `server` зависит от engine. Альтернатива —
оставить плоскую структуру и договориться «не импортировать вверх» —
отклонена: без физического разделения правило не соблюдается.

### 2. Интерфейсы как граница

```go
// engine определяет, runtime реализует
type Executor interface {
    Run(ctx context.Context, sc StageContext) (Result, error)
}
type GateRunner interface {
    Check(ctx context.Context, g Gate, sc StageContext) (Decision, error)
}
type EventSink interface {
    Append(ctx context.Context, ev Event) error
}
```

Engine зависит только от этих типов. Конкретные исполнители (agent CLI,
external program, Docker) регистрируются в `cmd`. Альтернатива — коллбеки/
функциональные опции — отклонена: интерфейсы дают явный контракт и тестируемость.

### 3. Единый StageContext и Result

Один `StageContext{Ctx, RunID, StageID, Paths, Logger}` вместо 6 параметров.
Один `Result{Status, Evidence, Message}` вместо разрозненных
`verdict`/`outcome`/`status`. Старые типы алиасим на переходный период.

### 4. Каталог гейтов = 4 типа

`program` (внешняя программа/Docker), `llm`, `human`, `required_phrases`.
Остальные встроенные типы — конфигурации `program`. Реестр гейтов остаётся,
но регистрация нового типа = реализация `Gate` + одна строка в реестре (OCP).

### 5. Инкрементальная миграция

Шаги независимы, каждый — отдельный PR:
1. Ввести интерфейсы + `StageContext`/`Result`, адаптировать существующие
   вызовы через тонкие обёртки (поведение не меняется).
2. Свести gate-пакеты в `engine/gate/` (git mv + fix imports).
3. Свести pipeline+workflow+control+approval в `engine/`.
4. Свести stage-пакеты в `engine/stage/`.
5. Вынести адаптеры в `runtime/`, разорвать импорты вверх.
6. Свести evidence/verdict/runid в `store/evidence/`.
7. `workaistrator init` + встроенный demo + docs.

## Risks / Trade-offs

- [Массовый git mv ломает импорты и историю] → делаем `git mv` + автозамену
  импортов, прогон `make build && make test` на каждом шаге; история
  сохраняется через `--follow`.
- [Алиасы типов на переходный период оставят дубли] → помечаем deprecated,
  удаляем на последнем шаге отдельным PR.
- [Скрытые циклические зависимости вскроются при разделении] → это цель:
  разрываем через интерфейсы, а не обходим.
- [Рост числа PR] → осознанная цена за отсутствие big-bang; каждый PR маленький
  и обратимый.

## Open Questions

- Оставляем ли `pkg/server` или сохраняем имя `pkg/web` для меньшего diff?
  Предложение: сохранить `web`, переименование — отдельный косметический PR.
- Demo-workflow: встраивать через `embed` в бинарник или генерировать `init`?
  Предложение: `embed` — работает без записи на диск.
