## MODIFIED Requirements

### Requirement: E2E-тест полного пайплайна
Система MUST иметь тест, проверяющий `workaistrator run` от начала до конца, включая
первый запуск сразу после инициализации.

#### Scenario: Успешный pipeline после init
- **КОГДА** E2E-тест создаёт чистый Git project и запускает `workaistrator init`
- **ТОГДА** `git status --porcelain` MUST остаться пустым
- **И** тест MUST запустить `workaistrator run` без ручного изменения или коммита
  `.gitignore`
- **И** baseline guard MUST принять workspace
