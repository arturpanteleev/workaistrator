# Proposal: Simplify WorkAiStrator Orchestrator

## Why

WorkAiStrator вырос до ~25 пакетов с размытыми границами: логика гейтов, этапов,
pipeline и workflow размазана по `gatelib`, `gaterun`, `gatesched`, `stageexec`,
`pipeline`, `workflow`, `control`. Это затрудняет онбординг, повышает связность
и мешает воспринимать продукт как то, чем он является — **движок графовых
workflow для оркестрации агентов**. Упрощаем архитектуру, не меняя семантику
ядра: граф, переходы, гейты, доказательства.

## What Changes

- Ввести три слоя с однонаправленными зависимостями: `engine/` (ядро),
  `runtime/` (адаптеры LLM/Docker/git/Jira), `store/` (состояние). Engine не
  импортирует runtime и store — связь через интерфейсы.
- Объединить пакеты гейтов (`gatelib`, `gaterun`, `gatesched`, `gatephrases`,
  `llmgate`, `humandecision`) в `engine/gate/`.
- Объединить исполнение этапов (`stageexec`, `stagestatus`, `dockerrun`,
  `checks`) в `engine/stage/`.
- Свести `pipeline`, `workflow`, `control`, `approval` в единую машину
  состояний `engine/`.
- Свести `evidence`, `verdict`, `runid` в `store/evidence/`.
- Вынести опциональные адаптеры (Jira, OIDC, TeamCity, MCP) за интерфейсы;
  ядро не тянет их зависимости.
- Ввести единый `StageContext` и единый тип результата вместо разрозненных
  `verdict`/`outcome`/`status` (DRY).
- Сократить набор встроенных типов гейтов до 4: `program`, `llm`, `human`,
  `required_phrases`; остальное — частные случаи `program`.
- Упростить онбординг: `workaistrator init`, встроенный demo-workflow,
  документация из 3 страниц (Quickstart, Workflow Reference, Deployment).

## Capabilities

### New Capabilities
- `engine-core`: единый движок графовых workflow — определение графа, машина
  состояний, переходы, исполнение этапов через интерфейсы `Executor`,
  `GateRunner`, `EventSink`.
- `onboarding`: `workaistrator init`, встроенный demo-workflow без внешних
  зависимостей, минимальный Quickstart.

### Modified Capabilities
- `process-template`: единая YAML-модель workflow (stages + transitions) как
  единственный источник истины о графе.
- `stage-executor`: исполнение этапа только через интерфейс `Executor`;
  внутренний агент, внешняя программа и Docker — реализации одного контракта.
- `gate-contract`: единый интерфейс `Gate` для всех проверок; каталог типов
  сокращён до `program`, `llm`, `human`, `required_phrases`.

## Impact

- Затронуты пакеты `pkg/gatelib`, `pkg/gaterun`, `pkg/gatesched`,
  `pkg/gatephrases`, `pkg/llmgate`, `pkg/humandecision`, `pkg/stageexec`,
  `pkg/stagestatus`, `pkg/dockerrun`, `pkg/checks`, `pkg/pipeline`,
  `pkg/workflow`, `pkg/control`, `pkg/approval`, `pkg/evidence`, `pkg/verdict`,
  `pkg/runid`, `pkg/web`.
- Публичный Go API меняется (объединение пакетов) — внутренний рефакторинг,
  CLI-контракт и YAML-формат workflow сохраняются.
- Семантика выполнения, гейтов, доказательств и delivery не меняется.
