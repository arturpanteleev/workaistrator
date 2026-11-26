# onboarding

## ADDED Requirements

### Requirement: Команда инициализации проекта
CLI SHALL предоставлять команду `workaistrator init`, которая генерирует
стартовый `workflow.yaml` и `.env.example` с комментариями в текущей
директории.

#### Scenario: Инициализация в пустой директории
- **WHEN** пользователь выполняет `workaistrator init` в пустой директории
- **THEN** создаются `workflow.yaml` с валидным графом и `.env.example`, и
  команда `workaistrator run --workflow workflow.yaml` проходит валидацию

### Requirement: Встроенный demo-workflow
Бинарник SHALL содержать встроенный demo-workflow, исполняемый без внешних
зависимостей (ключей API, Docker, git-репозитория).

#### Scenario: Запуск demo без настройки
- **WHEN** пользователь выполняет `workaistrator run --demo` на машине без
  ключей и Docker
- **THEN** движок проходит все этапы demo-графа и завершается успешно, используя
  агентов-заглушки

### Requirement: Минимальная документация
Документация SHALL состоять из трёх основных страниц: Quickstart, Workflow
Reference, Deployment.

#### Scenario: Навигация по документации
- **WHEN** пользователь открывает сайт документации
- **THEN** верхнеуровневое меню содержит Quickstart, Workflow Reference и
  Deployment, а Quickstart ведёт к первому успешному прогону за ≤5 минут
