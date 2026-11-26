## ДОБАВЛЕННЫЕ Требования

### Requirement: E2E-тест полного пайплайна
Система MUST иметь тест, проверяющий `workaistrator run` от начала до конца.

#### Scenario: Успешный пайплайн
- **КОГДА** E2E-тест запускается
- **ТОГДА** он создаёт временный проект
- **И** запускает `workaistrator init`
- **И** запускает `workaistrator run --feature "e2e-test" --task "test"`
- **И** проверяет, что все артефакты созданы: proposal.md, specs/, design.md, tasks.md, review.md, test-report.md
- **И** проверяет exit code == 0

#### Scenario: Пайплайн падает при REJECTED review
- **КОГДА** mock opencode для Reviewer возвращает CHANGES_REQUESTED
- **ТОГДА** пайплайн MUST остановиться на Reviewer
- **И** exit code MUST быть != 0

#### Scenario: Init проверяет структуру
- **КОГДА** `workaistrator init` запускается
- **ТОГДА** E2E-тест проверяет, что созданы `.ai-team/config.yaml` и все директории артефактов
