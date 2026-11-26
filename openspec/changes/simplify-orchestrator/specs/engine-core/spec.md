# engine-core

## ADDED Requirements

### Requirement: Слоистая архитектура с однонаправленными зависимостями
Система SHALL быть организована в три слоя: `engine/` (ядро графа и машины
состояний), `runtime/` (адаптеры исполнения) и `store/` (состояние). Пакет
`engine/` MUST NOT импортировать `runtime/` или `store/`; связь MUST
осуществляться только через интерфейсы, определённые в `engine/`.

#### Scenario: Engine не зависит от адаптеров
- **WHEN** анализируются импорты пакета `pkg/engine/...`
- **THEN** среди них отсутствуют импорты `pkg/runtime/...` и `pkg/store/...`

#### Scenario: Адаптер реализует интерфейс engine
- **WHEN** добавляется новый адаптер исполнения в `pkg/runtime/`
- **THEN** он компилируется, реализуя интерфейс `engine.Executor`, без правок
  в `pkg/engine/`

### Requirement: Единые интерфейсы границы
Движок SHALL определять интерфейсы `Executor`, `GateRunner` и `EventSink` как
единственные точки взаимодействия с исполнением этапов, проверками и записью
событий.

#### Scenario: Исполнение этапа через Executor
- **WHEN** движок запускает этап
- **THEN** он вызывает `Executor.Run(ctx, StageContext)` и не обращается к
  конкретной реализации напрямую

### Requirement: Единый StageContext и Result
Все исполнители и гейты SHALL принимать единый `StageContext` и возвращать
единый `Result` вместо разрозненных типов verdict/outcome/status.

#### Scenario: Передача контекста этапа
- **WHEN** вызывается исполнитель или гейт
- **THEN** ему передаётся `StageContext{Ctx, RunID, StageID, Paths, Logger}`,
  а результат представлен типом `Result{Status, Evidence, Message}`
