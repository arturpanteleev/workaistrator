# Workflow Reference: stages, transitions, gates

Workflow — это граф. Один YAML-файл полностью описывает процесс: упорядоченные
этапы, переходы между ними и проверки, которые разрешают переход. Движок не
знает про конкретный сценарий — новый процесс описывается файлом, без правок
кода.

> [!NOTE]
> Эта страница описывает **целевую модель** workflow (задачи 2.3/4.x). Сегодня
> `workaistrator run` читает граф из `.ai-team/config.yaml`, а встроенный
> загрузчик онбординга (`workaistrator init`, `run --demo`) принимает только
> минимальную схему `schema_version: 1` с полями `id`, `title`, `agent` и
> `transitions`. Поля `executor`, `result`, `gates`, `max_visits`, `delivery`
> и раздел «Валидация» ниже описывают будущую единую схему и пока не
> принимаются загрузчиком.

> [!LEARN]
> - из чего состоит stage и как задать переходы `on_pass`/`on_fail`;
> - четыре типа гейтов: `program`, `llm`, `human`, `required_phrases`;
> - как описать loopback и защитить его `max_visits`;
> - что будет валидироваться до запуска.

## Минимальный workflow (целевая схема)

```yaml
name: feature-flow
title: Фича от задачи до ревью

stages:
  - id: intent
    title: Задача
    executor: human
    result: approve
    transitions: { on_pass: implementation }

  - id: implementation
    title: Реализация
    executor: agent
    agent: coder
    result: md
    transitions: { on_pass: done }
```

Целевая команда — `workaistrator run --feature add-login --task "..."`: сегодня
`run` берёт граф из `.ai-team/config.yaml` и не принимает флаг `--workflow`.

## Этап

| Поле | Обязательно | Что задаёт |
|---|---|---|
| `id` | да | уникальный ID этапа; на него ссылаются переходы |
| `title` | нет | название для людей |
| `executor` | да | кто исполняет: `human`, `agent` или имя внешнего адаптера |
| `agent` | для `agent` | агент-контракт этапа: входы, выходы, вердикт, разрешённые пути |
| `result` | да | тип результата: `md`, `link` или `approve` |
| `gates` | нет | проверки результата; см. ниже |
| `transitions` | нет | `on_pass` и `on_fail` |
| `max_visits` | нет | сколько раз этап может быть посещён в одном прогоне (защита циклов) |
| `delivery` | нет | требования delivery для PR-этапа: `require_checks`, `require_verdicts` |

`executor: human` — человек работает по уже готовым входам; отсутствующий вход
не ошибка, пока этап не переключили на агента. `executor: agent` запускает
внутреннего агента или внешнюю программу, если у этапа указан `adapter`.

## Transitions

Переход выбирает **контроллер** после того, как этап закончил исполнитель и
прошли обязательные гейты. Модель не решает, куда идти дальше.

| Правило | Когда срабатывает | По умолчанию |
|---|---|---|
| `on_pass` | этап успешен, обязательные гейты прошли | следующий этап по порядку |
| `on_fail` | этап провален или обязательный гейт вернул `fail` | прогон останавливается (`blocked`) |

- Значение — `id` другого этапа или `done` (терминальное состояние).
- `on_fail` на самого себя — повтор этапа; счётчик посещений растёт.
- Рекомендательный гейт (`mode: advisory`) не меняет переход, даже если упал.
- Неизвестный `id`, недостижимый этап и цикл без `max_visits` отклоняются при
  валидации.

### Loopback

Типичный цикл — ревью вернуло код на доработку:

```yaml
stages:
  - id: implementation
    title: Реализация
    executor: agent
    agent: coder
    result: md
    max_visits: 3
    transitions:
      on_pass: code_review
      on_fail: implementation

  - id: code_review
    title: Код-ревью
    executor: agent
    agent: reviewer
    result: md
    gates:
      - type: human
        params: { approver: product_owner, on_reject: stop }
    transitions:
      on_pass: qa
      on_fail: implementation   # возврат на доработку
```

Когда `max_visits` исчерпан, прогон останавливается как `blocked`: дальнейшее
решение принимает человек, автоматического четвёртого круга не будет.

## Гейты

Гейт проверяет одну неизменяемую версию результата (по её SHA-256) и
возвращает `pass`, `fail` или `error`. Встроенных типов четыре; проверка,
которой нет среди них, описывается как `program`.

Общие поля гейта:

| Поле | Что задаёт |
|---|---|
| `type` | `program`, `llm`, `human` или `required_phrases` |
| `name` | имя экземпляра, если на этапе несколько гейтов одного типа |
| `mode` | `required` (по умолчанию) или `advisory` |
| `params` | параметры конкретного типа |

Гейты этапа проверяют один и тот же результат: провал любого обязательного
гейта запускает `on_fail`. `error` — технический сбой, его можно повторить на
той же версии результата, не перезапуская исполнителя.

### `required_phrases` — проверка текста

```yaml
gates:
  - type: required_phrases
    params:
      phrases: ["Критерии приёмки", "метрика успеха"]
      case: insensitive          # sensitive | insensitive
      scope: section             # result (весь артефакт) | section
      section: "Критерии приёмки"
      match: all                 # all | any | at_least
      unit: word                 # substring (по умолчанию) | word
```

`min_count` обязателен при `match: at_least`. `regex: true` включает Go RE2
вместо буквального сравнения. Фразы до 50, каждая до 200 байт; пустой результат
и отсутствующий раздел — `fail`, нечитаемый файл — `error`.

### `program` — внешняя программа или Docker

```yaml
gates:
  - type: program
    params:
      command: ["./scripts/check-api.sh"]   # argv, без shell
      timeout: 5m
      cwd: .
      env_allow: [PATH, CI]
```

Программа получает JSON-запрос в stdin и возвращает один JSON-ответ в stdout;
ненулевой код выхода, таймаут и невалидный JSON — `error`. Контейнерный вариант
— то же самое плюс закреплённый образ:

```yaml
    params:
      image: alpine@sha256:<64 hex>
      command: ["/check.sh"]
      network: none
```

Протокол, лимиты вывода и правила изоляции — в
[Протоколе гейта](../reference/gate-protocol.md).

### `llm` — проверка агентом

```yaml
gates:
  - type: llm
    params:
      agent: verifier
      mode: grill      # раунды вопросов; без mode — один проход
      max_rounds: 3
```

LLM-гейт получает результат этапа и выносит вердикт по своему контракту. Это
такая же проверка, как остальные: её вердикт читает контроллер.

### `human` — решение человека

```yaml
gates:
  - type: human
    params:
      approver: product_owner   # функция, которая вправе подтвердить
      on_reject: stop           # stop | stay
```

`on_reject: stop` останавливает прогон при отказе, `stay` оставляет этап
открытым для новой версии. Человеческое подтверждение — решение о переходе,
поэтому оно не бывает входным гейтом. Подтверждать может только роль,
отличная от исполнителя этапа.

## Валидация (целевая схема)

В целевой модели граф проверяется до первого исполнения. Эти проверки появятся
вместе с единым загрузчиком; сегодня загрузчик онбординга проверяет
`schema_version: 1`, уникальность `id`, существование `entry`/`on_pass`/`on_fail`
и наличие терминального этапа. Целевые правила отклоняют запуск:

- переход на неизвестный `id` или в недостижимый этап;
- цикл без `max_visits`;
- неизвестный `type` гейта, лишний или отсутствующий `params`;
- несколько версий одного гейта в одной цепочке;
- `human`-гейт на входе этапа;
- `executor: agent` без агента или с агентом, которого нет в `workaistrator list`.

Сообщение называет путь: `stages[2] (code_review): gates[0]: ...`.

## Совместимость с текущим конфигом

Канонический `workflow.yaml` ещё не подключён к `run`: сегодня `run` читает
`.ai-team/config.yaml` (schema 6/7), где та же модель записана эквивалентно:

| Канонически | Сейчас в `.ai-team/config.yaml` |
|---|---|
| `transitions.on_pass` | порядок `stages[]` |
| `transitions.on_fail` | `returns: [{from, to}]` |
| `gates` | `exit_gate_groups` (входные — `entry_gate_groups`) |
| `type: human` | `human_approval@N` |
| `type: llm` | `agent_review@N` |
| `type: program` | `external_program@1`, `docker_program@1` |
| `type: required_phrases` | `required_phrases@2` |

Полный справочник полей и профилей — в [Конфигурации](../reference/config.md).
