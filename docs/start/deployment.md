# Deployment: один бинарник и архив доказательств

WorkAiStrator — один Go-бинарник. Внутри: движок графа, SQLite-база дашборда и
собранный фронтенд. Внешняя база, брокер и отдельный сервис не нужны; для
работы агентов нужен только их CLI-рантайм, а для delivery — авторизованный
`gh`.

> [!LEARN]
> - как поднять дашборд локально и для команды;
> - как включить вход по токену и через OIDC;
> - где лежит состояние и архив evidence;
> - как делать backup, retention и обновление.

## Сборка

```bash
go build -o bin/workaistrator ./cmd/workaistrator
install -m 0755 bin/workaistrator /usr/local/bin/workaistrator
workaistrator version
```

Релизные бинарники для darwin/linux × amd64/arm64 лежат в
[релизах](https://github.com/arturpanteleev/workaistrator/releases/latest) и
подписаны cosign (keyless, Sigstore), рядом — `sha256sums.txt`. Как проверить
подпись — в [установке](../tutorial/install.md).

## Локальный режим

Для одного разработчика и одного репозитория:

```bash
cd /srv/my-project
workaistrator web
```

Откройте <http://127.0.0.1:8080>. По умолчанию сервер слушает только loopback,
сам выдаёт сессию и не требует пароля. Токен доступа лежит в
`.ai-team/web.token` — не коммитьте его.

```bash
workaistrator web --target /srv/my-project --port 8080
```

## Командный режим

Для команды включите аутентификацию: без неё сервер откажется слушать
не-loopback адрес. Секрет — случайная строка не короче 32 байт.

```bash
export AI_TEAM_AUTH_SECRET='<секрет не короче 32 байт>'
workaistrator web --host 127.0.0.1 --port 8080 --target /srv/my-project
```

Терминацию TLS делайте на reverse proxy (nginx, Caddy, load balancer) и
проксируйте на loopback. Наружу без HTTPS дашборд не выставляйте: в нём
артефакты, логи и решения по коду.

Первого администратора и персональные токены выпускает `auth-token` — отдельной
командой, с тем же секретом в окружении:

```bash
workaistrator auth-token --actor architect-1 --roles architect,reviewer --ttl 1h
```

Токен нужен только для создания браузерной сессии; дальше работают сессии и
роли. `--ttl` — не больше `24h`.

### Вход через OIDC

```bash
export AI_TEAM_AUTH_SECRET='<секрет не короче 32 байт>'
export AI_TEAM_OIDC_ISSUER='https://login.example.com/realms/team'
export AI_TEAM_OIDC_CLIENT_ID='workaistrator'
export AI_TEAM_OIDC_CLIENT_SECRET='<секрет клиента>'
export AI_TEAM_OIDC_REDIRECT_URL='https://workaistrator.example.com/api/auth/oidc/callback'
workaistrator web --host 127.0.0.1 --target /srv/my-project
```

Пользователь определяется по подтверждённому e-mail, функции берутся из
назначений участника в команде, а не из claims токена. Смена
`AI_TEAM_AUTH_SECRET`, регистрации OIDC или отключение SSO отзывают все сессии.
Все адреса — только `https` (кроме loopback для разработки).

## Supervisor

Пример unit-файла; секреты — в `EnvironmentFile`, а не в `ExecStart`:

```ini
# /etc/systemd/system/workaistrator.service
[Unit]
Description=WorkAiStrator dashboard
After=network-online.target

[Service]
User=workaistrator
WorkingDirectory=/srv/my-project
EnvironmentFile=/etc/workaistrator.env
ExecStart=/usr/local/bin/workaistrator web --host 127.0.0.1 --port 8080
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Для нескольких репозиториев запускайте отдельный процесс на каждый `--target`
(и отдельный порт) либо используйте очередь: `--scheduler-db` и
`--worker-command` разносят приём заданий и исполнение. Подробности — в
[дашборде](../guides/dashboard.md).

## Где лежит состояние

Всё живёт в каталоге проекта:

| Путь | Что | Входит в backup |
|---|---|---|
| `.ai-team/config.yaml` | конфигурация и workflow | да |
| `.ai-team/runs/<run_id>/` | неизменяемые доказательства прогона | да |
| `.ai-team/state/` | точка остановки, approvals, usage | да |
| `.ai-team/web.db` | SQLite дашборда: проекты, задачи, сессии | да |
| `.ai-team/artifacts/` | рабочие артефакты агентов | по желанию |
| `.ai-team/worktrees/<run_id>/` | кандидат прогона | нет |
| `.ai-team/exports/` | переносимые пакеты evidence | по желанию |

`.ai-team/runs/` — архив доказательств: `run.json`, `events.jsonl` с цепочкой
хешей, `attempts/`, логи, delivery и attestation. Файлы нельзя редактировать;
`workaistrator verify <run_id>` проверяет якорь, цепочку и попытки.

Для передачи результата коллеге или аудитору соберите самодостаточный пакет:

```bash
workaistrator export <run_id>
workaistrator redact scan --run <run_id>   # секреты — до отправки
```

Подробно — в [доказательствах](../guides/evidence.md).

## Backup, retention, обновление

**Backup.** Остановите сервер либо снимите базу штатно и скопируйте каталоги:

```bash
workaistrator db backup --db .ai-team/web.db --out /backup/web.db
tar czf /backup/ai-team-state.tgz .ai-team/runs .ai-team/state .ai-team/config.yaml
```

**Retention.** Завершённые прогоны чистятся `workaistrator gc`; evidence
удаляется только с `--prune-runs` и только при наличии проверенного экспорта.
Умолчания (`720h`, `keep_last: 20`) задаются секцией `retention` в конфиге.

```bash
workaistrator gc --dry-run              # что будет удалено
workaistrator gc --older-than 2160h     # 90 дней
```

**Обновление.** Замените бинарник и перезапустите сервис: миграции SQLite
применяются при открытии базы, формат `.ai-team/` совместим. Перед обновлением
сделайте backup.

## Безопасность

- Движок не изолирует агентов: агент работает с правами своего процесса.
  Запускайте недоверенный код в одноразовых контейнерах, сетевая изоляция — на
  уровне инфраструктуры.
- Дашборд — это доступ к коду и решениям. Только HTTPS, короткие токены,
 минимально необходимые роли.
- Веб и MCP не могут подменить команды гейтов и инфраструктуру: они меняют
  только процессные поля, остальное — ревьюируемый YAML в репозитории.
- Не редактируйте `.ai-team/runs/` и `.ai-team/state/` — `verify` откажет.

Полный список гарантий и ограничений — в
[Границе безопасности](../reference/security.md); ограничения облачного режима —
в [cloud pilot](../guides/cloud-pilot.md).
