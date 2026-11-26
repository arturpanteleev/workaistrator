# process-template (delta)

## ADDED Requirements

### Requirement: Workflow YAML как единственный источник истины о графе
Один YAML-файл workflow SHALL полностью описывать граф: упорядоченные `stages`
и `transitions` с правилами `on_pass`/`on_fail` для каждого этапа. Добавление
нового сценария оркестрации MUST NOT требовать изменений в коде движка.

#### Scenario: Новый сценарий без правок кода
- **WHEN** пользователь описывает новый граф в YAML с stages и transitions
- **THEN** движок валидирует и исполняет его без перекомпиляции и без правок в
  `pkg/engine/`

#### Scenario: Loopback через transitions
- **WHEN** у этапа задано `transitions: { on_fail: <id предыдущего этапа> }`
- **THEN** при провале гейта движок возвращает выполнение на указанный этап
