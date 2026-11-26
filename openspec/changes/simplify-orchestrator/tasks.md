# Tasks: Simplify WorkAiStrator Orchestrator

## 1. Интерфейсы и единые типы

- [x] 1.1 Создать `pkg/engine/` с интерфейсами `Executor`, `GateRunner`, `EventSink`
- [x] 1.2 Ввести `StageContext{Ctx, RunID, StageID, Paths, Logger}` и `Result{Status, Evidence, Message}`
- [x] 1.3 Адаптировать существующие вызовы через тонкие обёртки (поведение не меняется)
- [x] 1.4 Прогнать `make build && make test`

## 2. Объединение гейтов

- [x] 2.1 Перенести `gatelib`, `gaterun`, `gatesched`, `gatephrases`, `llmgate`, `humandecision` в `pkg/engine/gate/` (git mv + fix imports)
- [x] 2.2 Свести все гейты к реализации `GateRunner`; реестр регистрации без switch
- [x] 2.3 Сократить встроенные типы до `program`, `llm`, `human`, `required_phrases`
- [x] 2.4 Прогнать `make build && make test`

## 3. Единая машина состояний

- [x] 3.1 Перенести `pipeline`, `workflow`, `control`, `approval` в `pkg/engine/`
- [x] 3.2 Устранить циклические импорты через интерфейсы
- [x] 3.3 Прогнать `make build && make test`

## 4. Исполнение этапов

- [x] 4.1 Перенести `stageexec`, `stagestatus`, `dockerrun`, `checks` в `pkg/engine/stage/`
- [x] 4.2 Все исполнители реализуют `Executor`; выбор по имени из реестра
- [x] 4.3 Прогнать `make build && make test`

## 5. Адаптеры и состояние

- [x] 5.1 Вынести LLM/Docker/git/Jira в `pkg/runtime/`, реализующие интерфейсы engine
- [x] 5.2 Свести `evidence`, `verdict`, `runid` в `pkg/store/evidence/`
- [ ] 5.3 Разорвать импорты engine → runtime/store (проверка через `go list`)
  - Проверено: ядро (`pkg/engine/*.go`, `gate/`, `stage/`) не импортирует
    runtime/store; оставшиеся переходы живут в `engine/pipeline` (унаследованная
    оркестрация: `token_budget.go`, `stage_executor.go`, `gate_wave.go`,
    `human_stage.go`, `execute.go`, `finalize.go` и др., плюс тесты).
    Полный разрыв требует перенести контракты `runtime`/`store` в engine и
    является отдельным проходом, чтобы не менять поведение.
- [x] 5.4 Прогнать `make build && make test`

## 6. Онбординг

- [x] 6.1 Реализовать `workaistrator init` (генерация workflow.yaml + .env.example)
- [x] 6.2 Встроить demo-workflow через embed; команда `workaistrator run --demo`
- [x] 6.3 Свести документацию к Quickstart, Workflow Reference, Deployment
- [x] 6.4 Обновить README: одна команда, одна картинка графа

## 7. Финал

- [x] 7.1 Удалить deprecated-алиасы типов
- [x] 7.2 Прогнать `make verify` (race/security/frontend)
  - Зелёный после обновления `gorilla/websocket` до исправленного коммита
    `v1.5.4-0.20240701034025-d67f41855da4` (GO-2026-6278; уязвимость была ещё
    в `master`, stable-релиза с фиксом нет).
- [x] 7.3 Обновить CLAUDE.md и docs/ARCHITECTURE.md под новую структуру

## 8. Отложенный долг (после релиза)

- [ ] 8.1 Разорвать оставшиеся импорты `pkg/engine/pipeline` → `pkg/runtime`, `pkg/store`: вынести используемые типы в интерфейсы ядра и перевести.repository на адаптеры (5.3).
- [ ] 8.2 Выровнять имена пакетов с каталогами (`gate/lib` = `gatelib`, `stage/exec` = `stageexec`) альясами или переименованием.
