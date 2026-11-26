# WorkAiStrator

[![CI](https://github.com/arturpanteleev/workaistrator/actions/workflows/ci.yaml/badge.svg)](https://github.com/arturpanteleev/workaistrator/actions/workflows/ci.yaml)
[![Docs](https://img.shields.io/badge/docs-site-blue)](https://arturpanteleev.github.io/workaistrator/)
[![Release](https://img.shields.io/github/v/release/arturpanteleev/workaistrator)](https://github.com/arturpanteleev/workaistrator/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/arturpanteleev/workaistrator)](https://go.dev/dl/)
[![License](https://img.shields.io/github/license/arturpanteleev/workaistrator)](LICENSE)

**Движок графовых workflow для оркестрации AI-агентов.** Этапы, переходы,
гейты, доказательства и delivery исполняет детерминированный контроллер на Go,
а не модель. Коммит, push и PR появляются только после того, как вы
подтвердили точный delivery-план по его SHA-256.

## Одна команда

Из клона репозитория — без ключей, Docker и настроек:

```bash
go build -o bin/workaistrator ./cmd/workaistrator && ./bin/workaistrator run --demo
```

Demo проходит граф на агентах-заглушках и ничего не пишет на диск: это
симуляция обхода графа, без `run_id` и без evidence. Настоящие доказательства
появляются только у реального прогона — см.
[Quickstart](docs/start/quickstart.md). Если в вашей сборке команды `run --demo`
ещё нет, тот же сценарий запускает `bash docs/demo/run-demo.sh`.

## Как выглядит workflow

```text
intent ─▶ product_spec ─▶ tech_design ─▶ implementation ─▶ code_review ─▶ qa ─▶ done
                                        ▲                                │
                                        └─────────── on_fail ────────────┘

on_fail может вести на любой предыдущий этап; цикл защищён max_visits.
```

Граф описывается одним YAML: `stages`, `transitions` (`on_pass`/`on_fail`) и
гейты четырёх типов — `program`, `llm`, `human`, `required_phrases`.

## Документация

Начните с трёх страниц:

| Страница | Зачем |
|---|---|
| **[Quickstart](docs/start/quickstart.md)** | от нуля до первого успешного прогона за ≤5 минут |
| **[Workflow Reference](docs/start/workflow-reference.md)** | stages, transitions, гейты и loopback |
| **[Deployment](docs/start/deployment.md)** | один бинарник, дашборд, OIDC, архив evidence |

Полный справочник:

| Раздел | Что внутри |
|---|---|
| [Ключевые понятия](docs/start/concepts.md) | прогон, кандидат, вердикт, delivery-план; глоссарий |
| [Как это выглядит](docs/start/tour.md) | один прогон от задачи до PR в скриншотах |
| [Команды CLI](docs/reference/cli.md) | все команды и флаги |
| [Конфигурация](docs/reference/config.md) | `.ai-team/config.yaml`, профили, рантаймы и ключи |
| [Агенты и этапы](docs/reference/pipeline.md) | кто что делает и где решает человек |
| [Статусы и коды выхода](docs/reference/statuses.md) | что означает код и что делать |
| [Граница безопасности](docs/reference/security.md) | что гарантируется, а что нет |
| [Протокол гейта](docs/reference/gate-protocol.md) | JSON-контракт гейта внешней программы |
| [Сетевой MCP](docs/reference/mcp.md) | подключение Claude и Codex, токены, паритет с REST |
| [Web API задач](docs/reference/TASK-FLOW-WEB-API.md) | REST-маршруты, статусы этапов, гейты, права |
| [Архитектура](docs/ARCHITECTURE.md) | как устроен контроллер внутри |

## Зачем это

С обычным coding-агентом трудно быть уверенным, что между «агент написал код»
и «код попал в main» были ревью и тесты, а не «похоже, ок». WorkAiStrator
превращает это в протокол:

- каждый этап заканчивается вердиктом, который читает контроллер;
- агент меняет только разрешённые пути, остальное проваливает этап;
- проверки запускает контроллер, а не модель;
- delivery идёт ровно по подтверждённому плану;
- каждый прогон оставляет проверяемые доказательства.

## Для кого

Для команды, которой нужен предсказуемый процесс вокруг AI-агентов: аналитик,
архитектор, программист, ревьюер, тестировщик. WorkAiStrator разворачивается
на сервере или в облаке, команда работает в одном дашборде и подтверждает
переходы по своим ролям. Попробовать можно на своей машине с одним
репозиторием.

> [!NOTE]
> WorkAiStrator не изолирует агентов сам: агент работает с правами процесса,
> который его запустил. В облаке запускайте исполнителей в одноразовых
> контейнерах. Подробно — в [Границе безопасности](docs/reference/security.md).

## Установка

Нужны Go 1.26.11+ (для `go install`), один CLI-рантайм в `PATH`
([OpenCode](https://opencode.ai), [Codex](https://github.com/openai/codex) или
[Claude Code](https://docs.anthropic.com/en/docs/claude-code)) и, для delivery,
авторизованный [`gh`](https://cli.github.com). Для demo и `workaistrator gate`
ничего этого не нужно.

```bash
go install github.com/arturpanteleev/workaistrator/cmd/workaistrator@latest
workaistrator version
```

Готовые бинарники — в
[релизах](https://github.com/arturpanteleev/workaistrator/releases/latest):
darwin/linux × amd64/arm64, подпись cosign (keyless, Sigstore) и
`sha256sums.txt`.

## Быстрый старт

```bash
workaistrator init
workaistrator run --feature add-login \
  --task "Добавить вход по e-mail и паролю"
```

`init` создаёт `.ai-team/config.yaml` с рабочим графом; `run` читает граф
оттуда. Перед delivery контроллер покажет план и его SHA-256 и спросит
подтверждение.
Без терминала прогон остановится с кодом `3`; подтвердите именно этот план:

```bash
workaistrator run --resume <run_id> --approve-plan <sha256>
```

Результат — в дашборде `workaistrator web` на <http://127.0.0.1:8080> и в
`.ai-team/runs/<run_id>/`. Пошагово — в [Quickstart](docs/start/quickstart.md).

Ключ провайдера агент видит, только если вы разрешили его имя:
`AI_TEAM_HARNESS_ENV_ALLOW=ANTHROPIC_API_KEY workaistrator run …`.

## Дашборд и возможности

`workaistrator web` поднимает дашборд: проекты, задачи, графы, решения,
артефакты и доказательства. Для команды — облачный режим с токенами и SSO по
OIDC, роли в подтверждениях, очередь заданий. Гейты работают на входе и выходе
этапа; группа гейтов идёт последовательно, гейты внутри группы — параллельно;
проваленный обязательный гейт блокирует переход, обход доступен только
администратору и виден в журнале. Сетевой MCP даёт Claude и Codex те же
чтения и действия, что Web/REST. Трекер задач двигает граф только через
контроллер.

Развёртывание, вход, backup и evidence — в [Deployment](docs/start/deployment.md).

## Разработка

Нужны Go 1.26.11+ и Node 22+ (фронтенд дашборда); точная версия Go закреплена
в [`.tool-versions`](.tool-versions).

```bash
make build          # сборка bin/workaistrator
make test           # go test ./...
make test-e2e       # E2E на mock-opencode
make specs          # строгая проверка OpenSpec
make docs           # сайт документации в docs/_site
make verify         # полная проверка, как в CI
```

Как устроен процесс и что проверяет CI — в [«Как участвовать»](CONTRIBUTING.md)
и [«Разработка»](docs/DEVELOPMENT.md). Ошибки и предложения — в
[GitHub Issues](https://github.com/arturpanteleev/workaistrator/issues).
Уязвимости — только приватно, по [«Сообщить об уязвимости»](SECURITY.md).

Сайт документации собирается генератором [`docsgen/`](docsgen/) из Markdown
этого репозитория и публикуется на GitHub Pages при push в `master`. Релизы
собираются по тегу `v*`: CI против коммита тега, затем сборка, подпись cosign и
публикация.

## Лицензия

Apache License 2.0 — см. [LICENSE](LICENSE).
