## ADDED Requirements

### Requirement: Команда workaistrator web
Система MUST предоставить CLI команду для запуска web-сервера.

#### Scenario: Запуск с портом
- **КОГДА** пользователь запускает `workaistrator web --port 9090`
- **ТОГДА** сервер MUST запуститься на порту 9090

#### Scenario: Запуск по умолчанию
- **КОГДА** пользователь запускает `workaistrator web`
- **ТОГДА** сервер MUST запуститься на порту 8080

#### Scenario: Информация при запуске
- **КОГДА** сервер запускается
- **ТОГДА** в консоль MUST вывестись: `Web UI available at http://localhost:8080`
