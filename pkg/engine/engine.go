// Package engine определяет ядро движка графовых workflow WorkAiStrator.
//
// Engine не импортирует runtime или store: связь с внешним миром идёт только
// через интерфейсы Executor, GateRunner и EventSink. Конкретные реализации
// (LLM-адаптеры, Docker, git, SQLite) регистрируются в cmd/workaistrator.
package engine

import (
	"context"
	"io"
	"log/slog"
)

// StageContext — единый контекст исполнения этапа. Передаётся исполнителям
// и гейтам вместо разрозненного набора параметров.
type StageContext struct {
	Ctx     context.Context
	RunID   string
	StageID string
	// Paths — разрешённые пути мутации (repository-relative globs).
	Paths  []string
	Logger *slog.Logger
	// Stdin/Stdout/Stderr опциональны; nil означает «не подключено».
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Status — результат этапа или гейта.
type Status string

const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
	// StatusBlocked означает, что этап не может продолжиться без вмешательства.
	StatusBlocked Status = "blocked"
)

// Result — единый результат исполнения этапа или гейта. Заменяет разрозненные
// verdict/outcome/status.
type Result struct {
	Status   Status
	Evidence string // путь к артефакту/доказательству, если есть
	Message  string
}

// Decision — решение гейта.
type Decision struct {
	Pass    bool
	Message string
}

// Executor исполняет этап: внутренний агент, внешняя программа или Docker.
// Реализации регистрируются по имени; движок не выбирает их через switch.
type Executor interface {
	Run(ctx context.Context, sc StageContext) (Result, error)
}

// Gate — определение гейта, переданное исполнителю проверки.
type Gate struct {
	ID      string
	Version int
	Kind    string // program | llm | human | required_phrases
	Params  map[string]any
}

// GateRunner выполняет одну проверку. Регистрация нового типа гейта =
// реализация интерфейса + одна запись в реестре.
type GateRunner interface {
	Check(ctx context.Context, g Gate, sc StageContext) (Decision, error)
}

// Event — событие журнала выполнения.
type Event struct {
	RunID   string
	StageID string
	Type    string
	Payload map[string]any
}

// EventSink принимает события выполнения (append-only журнал).
type EventSink interface {
	Append(ctx context.Context, ev Event) error
}
