## ADDED Requirements

### Requirement: Директория reports при инициализации
Система MUST создавать `.ai-team/reports/` при `workaistrator init`.

#### Scenario: Init создаёт reports
- **КОГДА** `workaistrator init` запускается
- **ТОГДА** система MUST создать `.ai-team/reports/` директорию
- **И** `.ai-team/reports/` MUST быть добавлена в `.gitignore`

### Requirement: Обновлённый конфиг по умолчанию
Конфиг по умолчанию MUST включать ролевые настройки.

#### Scenario: Структура конфига
- **КОГДА** `workaistrator init` запускается
- **ТОГДА** config.yaml MUST содержать:
  - `pipeline:` с полными именами агентов (например `[analyst, architect, coder, reviewer, tester, deployer]`)
  - `cli: opencode`
  - `model: auto`
  - `effort: medium`
