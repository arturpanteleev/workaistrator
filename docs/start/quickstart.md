# Quickstart: от нуля до первого прогона

Пять минут, один бинарник, без ключей и без модели: встроенный demo-workflow
проходит граф на агентах-заглушках. Дальше — та же команда на вашем проекте.

> [!LEARN]
> - собрать `workaistrator` и запустить demo;
> - инициализировать свой проект;
> - провести первую задачу и подтвердить delivery-план;
> - найти результат — в дашборде и в evidence.

## 1. Собрать бинарник

Нужен Go 1.26.11+ (точная версия для этого репозитория — в `.tool-versions`).
Больше ничего: SQLite и фронтенд дашборда встроены в бинарник.

```bash
git clone https://github.com/arturpanteleev/workaistrator.git
cd workaistrator
go build -o bin/workaistrator ./cmd/workaistrator
./bin/workaistrator version
```

Вместо сборки можно взять готовый бинарник из
[релизов](https://github.com/arturpanteleev/workaistrator/releases/latest) или
поставить `go install github.com/arturpanteleev/workaistrator/cmd/workaistrator@latest`.

## 2. Первый прогон: demo

```bash
./bin/workaistrator run --demo
```

Demo-граф проходит все этапы на заглушках: ни ключа провайдера, ни Docker, ни
Git-репозитория не требуется. Это симуляция обхода графа: она печатает этапы
и переходы, но не создаёт `run_id`, артефактов и evidence и ничего не пишет на
диск. Доказательства появляются у реального прогона — шаги 3–5.

> [!NOTE]
> `run --demo` появляется вместе с встроенным demo-workflow. Если в вашей
> сборке команды ещё нет — тот же сценарий без ключей запускает
> `bash docs/demo/run-demo.sh` (нужны `bash`, `git` и Go).

## 3. Свой проект

```bash
cd ~/my-project
workaistrator init
```

`init` создаёт `.ai-team/config.yaml` с рабочим графом этапов, `workflow.yaml`
с минимальным графом для будущего единого загрузчика и `.env.example` с
комментариями. Сегодня `run` читает граф из `.ai-team/config.yaml`; поправьте
этапы там под свою команду. Целевая схема `workflow.yaml` — в
[Workflow Reference](workflow-reference.md).

Запуск задачи:

```bash
workaistrator run \
  --feature add-login \
  --task "Добавить вход по e-mail и паролю"
```

> [!NOTE]
> `workflow.yaml` из `init` пока используется только встроенным demo и
> загрузчиком онбординга; `run` его не читает. Канонический `workflow.yaml`
> подключится к `run` вместе с единым загрузчиком движка.

Контроллер сам проверяет окружение (Git, ветка, доступность пуша) и ведёт
задачу по графу. Агент меняет только разрешённые пути, проверки запускает
контроллер, а не модель.

## 4. Подтвердить delivery

Перед коммитом контроллер собирает **delivery-план** — ветку, файлы и их хеши,
сообщение коммита, заголовок и описание PR — и останавливается.

- В терминале он спросит `Продолжить? [y/N]`.
- Без терминала (CI, скрипт) прогон завершится кодом `3` и напечатает план и
  его SHA-256.

Подтверждается ровно этот план:

```bash
workaistrator run --resume <run_id> --approve-plan <sha256>
```

Другой хеш не подойдёт: изменился план — подтверждайте заново. Только после
этого контроллер делает commit, push и открывает PR.

## 5. Где смотреть результат

| Что | Где |
|---|---|
| Прогон, этапы, решения | `workaistrator web` → <http://127.0.0.1:8080> |
| Доказательства прогона | `.ai-team/runs/<run_id>/` |
| Артефакты агентов | `.ai-team/artifacts/<feature>/` |
| HTML-отчёт | `.ai-team/reports/<feature>/index.html`; отчёты этапов — `.ai-team/reports/<feature>/attempts/<attempt_id>/index.html` |

Целостность доказательств проверяется командой:

```bash
workaistrator verify <run_id>
```

## Если нужна настоящая модель

Demo работает без модели. Для реальных прогонов нужен один CLI-рантайм в
`PATH` — [OpenCode](https://opencode.ai), [Codex](https://github.com/openai/codex)
или [Claude Code](https://docs.anthropic.com/en/docs/claude-code) — и ключ
провайдера, имя которого вы явно разрешили:

```bash
export AI_TEAM_HARNESS_ENV_ALLOW=ANTHROPIC_API_KEY
export ANTHROPIC_API_KEY=...
workaistrator run --feature add-login --task "..."
```

Агенты работают с правами процесса, который их запустил. Для недоверенного кода
запускайте исполнителей в одноразовых контейнерах — см.
[Границу безопасности](../reference/security.md).

## Дальше

- [Workflow Reference](workflow-reference.md) — stages, transitions, гейты.
- [Deployment](deployment.md) — дашборд для команды, OIDC, архив evidence.
- [Ключевые понятия](concepts.md) — прогон, кандидат, вердикт, delivery-план.
