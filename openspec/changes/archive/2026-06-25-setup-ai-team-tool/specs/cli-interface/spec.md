## ДОБАВЛЕННЫЕ Требования

### Requirement: Точка входа CLI
Система MUST предоставлять единый CLI-бинарник `workaistrator` с подкомандами.

#### Scenario: Вывод справки
- **КОГДА** пользователь запускает `workaistrator --help` или `workaistrator -h`
- **ТОГДА** система MUST вывести информацию об использовании со списком всех доступных команд

#### Scenario: Неизвестная команда
- **КОГДА** пользователь запускает `WorkAiStrator <неизвестная-команда>`
- **ТОГДА** система MUST вывести сообщение об ошибке и завершиться с ненулевым кодом

### Requirement: workaistrator init
Система MUST инициализировать `.ai-team/` в текущей директории.

#### Scenario: Init создаёт структуру директорий
- **КОГДА** пользователь запускает `workaistrator init`
- **ТОГДА** система MUST создать `.ai-team/config.yaml` с конфигом по умолчанию
- **И** создать `.ai-team/artifacts/product/`, `.ai-team/artifacts/tech/`, `.ai-team/artifacts/reviews/`
- **И** создать `.ai-team/artifacts/tasks/`

#### Scenario: Init проверяет OpenCode
- **КОГДА** пользователь запускает `workaistrator init`
- **ТОГДА** система MUST проверить, доступен ли `opencode` в PATH
- **И** вывести предупреждение, если не найден

### Requirement: workaistrator run
Система MUST выполнять пайплайн агентов для фичи.

#### Scenario: Запуск с флагом feature
- **КОГДА** пользователь запускает `workaistrator run --feature "add-auth" --task "Реализовать JWT авторизацию"`
- **ТОГДА** система MUST создать `.ai-team/artifacts/tasks/add-auth/task.md` с описанием задачи
- **И** выполнить пайплайн: Analyst → Architect → Coder → Reviewer → Tester → Deployer

#### Scenario: Остановка при ошибке агента
- **КОГДА** любой агент в пайплайне завершается с ошибкой
- **ТОГДА** система MUST остановить выполнение
- **И** вывести сообщение об ошибке с именем упавшего агента и причиной

### Requirement: workaistrator list
Система MUST выводить список всех доступных встроенных агентов.

#### Scenario: Список агентов
- **КОГДА** пользователь запускает `workaistrator list`
- **ТОГДА** система MUST вывести таблицу с именем агента, описанием и типом runtime

### Requirement: workaistrator version
Система MUST выводить текущую версию.

#### Scenario: Вывод версии
- **КОГДА** пользователь запускает `workaistrator version`
- **ТОГДА** система MUST вывести semver-строку версии
