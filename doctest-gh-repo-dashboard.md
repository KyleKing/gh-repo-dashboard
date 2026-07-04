# Documentation-Driven Testing Implementation Plan: gh-repo-dashboard

Implementation roadmap for adding fixture-based testing, auto-generated documentation, and vim-paradigm composability to gh-repo-dashboard.

## Vision

Transform gh-repo-dashboard into a fully composable, testable TUI using vim paradigm:
- **Text objects**: `dr` (dirty repos), `pr` (repos with PRs), `ar` (all repos), `br` (behind repos)
- **Operators**: `F` (fetch), `P` (prune), `C` (cleanup), `d` (delete/hide)
- **Composition**: `Fdr` (fetch dirty repos), `Cpr` (cleanup repos with PRs)
- **Predicates**: `:select where dirty and has_pr` (no visual mode needed)
- **Command mode**: `:filter dirty has_pr`, `:fetch marked`, `:mark 1,3,5-7`
- **Reusable framework**: Extract `tui-commander` package for all TUIs

## Current Architecture Analysis

**Existing patterns:**
- Bubble Tea model with `Update(msg tea.Msg)` pattern
- View modes: `ViewModeRepoList`, `ViewModeRepoDetail`, `ViewModeFilter`, etc.
- Key handling in `handleKey()`, `handleFilterKey()`, `handleDetailKey()`
- VCS abstraction (git/jj support via interface)
- Progressive loading with async messages
- TTL-based caching
- Batch task runner

**Testing gaps:**
- Tests coupled to keyboard simulation (see `internal/app/app_test.go`)
- State assertions after key presses, not semantic actions
- No documentation generated from tests
- Hard to replay or script operations
- wip-test-improvements.md mentions teatest/catwalk but not fixture-based

**Current state:** ~30% compatible with command-based architecture (needs more refactoring than jj-diff)

## Goals

1. **Composability**: vim-paradigm operators × text objects
2. **Testability**: Test semantic actions without keyboard simulation
3. **Documentation**: Auto-generate usage docs from test fixtures
4. **Reusability**: Extract `tui-commander` package for all TUIs
5. **Scriptability**: Command mode with auto-completion

## Phase 0: Reusable Framework (Week 1-2)

### 0.1 Create `tui-commander` Package

**Repository:** `github.com/kyleking/tui-commander`

**Structure:**
```
tui-commander/
├── go.mod
├── README.md
├── command.go          # Core Command interface
├── registry.go         # Command registration
├── parser.go           # Parse ":cmd args"
├── completion.go       # Auto-completion engine
├── component.go        # Bubble Tea component
├── predicates/
│   ├── parser.go       # Parse "dirty and has_pr"
│   ├── evaluator.go    # Evaluate predicates
│   ├── ast.go          # Predicate AST nodes
│   └── completion.go   # Field auto-completion
├── examples/
│   └── basic/          # Minimal example app
└── docs/
    └── DESIGN.md       # Framework design doc
```

### 0.2 Core Interfaces

**File:** `command.go`

```go
package commander

// Command represents an executable action
type Command interface {
    Execute(ctx CommandContext) error
    String() string           // For logging, replay, documentation
    Undo() (Command, error)   // Return inverse command if reversible
}

// CommandContext is implemented by app models
// Allows commands to access and modify app state
type CommandContext interface {
    GetState() interface{}
    SetState(interface{}) error
}

// CommandMsg wraps a command for Bubble Tea message passing
type CommandMsg struct {
    Cmd Command
}

// CommandExecutedMsg indicates command execution completed
type CommandExecutedMsg struct {
    Cmd   Command
    Error error
}
```

### 0.3 Command Registry

**File:** `registry.go`

```go
package commander

type Registry struct {
    commands map[string]CommandSpec
    aliases  map[string]string
}

type CommandSpec struct {
    Name        string
    Aliases     []string
    Args        []ArgSpec
    Description string
    Category    string  // "navigation", "filtering", "batch", etc.
    Factory     func(args []string) (Command, error)
}

type ArgSpec struct {
    Name         string
    Type         ArgType
    Required     bool
    Default      string
    Description  string
    Completions  CompletionFunc
}

type ArgType int

const (
    ArgTypeString ArgType = iota
    ArgTypeInt
    ArgTypeStringSlice  // Space-separated values
    ArgTypePredicate    // Predicate expression
    ArgTypeEnum         // Fixed set of values
    ArgTypeRange        // "1,3,5-7"
)

type CompletionFunc func(ctx CommandContext, partial string) []string

func NewRegistry() *Registry {
    return &Registry{
        commands: make(map[string]CommandSpec),
        aliases:  make(map[string]string),
    }
}

func (r *Registry) Register(spec CommandSpec) error {
    if _, exists := r.commands[spec.Name]; exists {
        return fmt.Errorf("command already registered: %s", spec.Name)
    }

    r.commands[spec.Name] = spec

    for _, alias := range spec.Aliases {
        r.aliases[alias] = spec.Name
    }

    return nil
}

func (r *Registry) Get(name string) (CommandSpec, bool) {
    if actualName, ok := r.aliases[name]; ok {
        name = actualName
    }
    spec, ok := r.commands[name]
    return spec, ok
}

func (r *Registry) AllCommands() []CommandSpec {
    specs := make([]CommandSpec, 0, len(r.commands))
    for _, spec := range r.commands {
        specs = append(specs, spec)
    }
    return specs
}

func (r *Registry) MatchingCommands(prefix string) []CommandSpec {
    var matches []CommandSpec
    for name, spec := range r.commands {
        if strings.HasPrefix(name, prefix) {
            matches = append(matches, spec)
        }
    }
    return matches
}
```

### 0.4 Command Parser

**File:** `parser.go`

```go
package commander

type Parser struct {
    registry *Registry
}

func NewParser(registry *Registry) *Parser {
    return &Parser{registry: registry}
}

// Parse parses command input with space-delimited arguments
// Examples:
//   "filter dirty has_pr" → SetFilterCmd{Modes: ["dirty", "has_pr"]}
//   "fetch marked" → FetchCmd{Target: "marked"}
//   "select where dirty and has_pr" → SelectCmd{Predicate: ...}
func (p *Parser) Parse(input string) (Command, error) {
    parts := strings.Fields(input)
    if len(parts) == 0 {
        return nil, ErrEmptyCommand
    }

    cmdName := parts[0]
    args := parts[1:]

    spec, ok := p.registry.Get(cmdName)
    if !ok {
        return nil, fmt.Errorf("unknown command: %s", cmdName)
    }

    // Validate and convert args
    if err := p.validateArgs(spec, args); err != nil {
        return nil, err
    }

    return spec.Factory(args)
}

func (p *Parser) validateArgs(spec CommandSpec, args []string) error {
    requiredCount := 0
    for _, argSpec := range spec.Args {
        if argSpec.Required {
            requiredCount++
        }
    }

    if len(args) < requiredCount {
        return fmt.Errorf("%s: expected at least %d args, got %d",
            spec.Name, requiredCount, len(args))
    }

    return nil
}

var ErrEmptyCommand = errors.New("empty command")
```

### 0.5 Auto-Completion

**File:** `completion.go`

```go
package commander

type Completion struct {
    Text        string
    Description string
    Category    string
    Highlight   string  // Part matching partial input
}

type CompletionContext struct {
    Input       string
    CursorPos   int
    AppContext  CommandContext
}

func (p *Parser) GetCompletions(ctx CompletionContext) []Completion {
    input := ctx.Input
    parts := strings.Fields(input)

    // No input - show all commands
    if len(parts) == 0 {
        return p.commandCompletions("")
    }

    cmdName := parts[0]

    // Partial command name
    if len(parts) == 1 && !strings.HasSuffix(input, " ") {
        return p.commandCompletions(cmdName)
    }

    // Complete arguments
    spec, ok := p.registry.Get(cmdName)
    if !ok {
        return nil
    }

    argIndex := len(parts) - 1
    if strings.HasSuffix(input, " ") {
        argIndex = len(parts)
    }

    if argIndex >= len(spec.Args) {
        return nil
    }

    argSpec := spec.Args[argIndex]
    partial := ""
    if argIndex < len(parts) {
        partial = parts[argIndex]
    }

    return p.argCompletions(ctx.AppContext, argSpec, partial)
}

func (p *Parser) commandCompletions(prefix string) []Completion {
    var completions []Completion

    for _, spec := range p.registry.AllCommands() {
        if prefix == "" || strings.HasPrefix(spec.Name, prefix) {
            completions = append(completions, Completion{
                Text:        spec.Name,
                Description: spec.Description,
                Category:    spec.Category,
                Highlight:   prefix,
            })
        }
    }

    return completions
}

func (p *Parser) argCompletions(ctx CommandContext, argSpec ArgSpec, partial string) []Completion {
    if argSpec.Completions == nil {
        return nil
    }

    suggestions := argSpec.Completions(ctx, partial)
    completions := make([]Completion, len(suggestions))

    for i, s := range suggestions {
        completions[i] = Completion{
            Text:        s,
            Description: argSpec.Description,
            Highlight:   partial,
        }
    }

    return completions
}
```

### 0.6 Predicate System

**File:** `predicates/ast.go`

```go
package predicates

// Predicate represents a boolean expression
type Predicate interface {
    Evaluate(item interface{}) bool
    String() string
}

// AndPred: left AND right
type AndPred struct {
    Left, Right Predicate
}

func (p AndPred) Evaluate(item interface{}) bool {
    return p.Left.Evaluate(item) && p.Right.Evaluate(item)
}

func (p AndPred) String() string {
    return fmt.Sprintf("(%s and %s)", p.Left, p.Right)
}

// OrPred: left OR right
type OrPred struct {
    Left, Right Predicate
}

func (p OrPred) Evaluate(item interface{}) bool {
    return p.Left.Evaluate(item) || p.Right.Evaluate(item)
}

func (p OrPred) String() string {
    return fmt.Sprintf("(%s or %s)", p.Left, p.Right)
}

// NotPred: NOT inner
type NotPred struct {
    Inner Predicate
}

func (p NotPred) Evaluate(item interface{}) bool {
    return !p.Inner.Evaluate(item)
}

func (p NotPred) String() string {
    return fmt.Sprintf("not %s", p.Inner)
}

// FieldPred: checks boolean field
type FieldPred struct {
    Field string
    Spec  FieldSpec
}

func (p FieldPred) Evaluate(item interface{}) bool {
    value := p.Spec.Getter(item)
    if b, ok := value.(bool); ok {
        return b
    }
    return false
}

func (p FieldPred) String() string {
    return p.Field
}

// ComparisonPred: field OP value
type ComparisonPred struct {
    Field string
    Op    CompareOp
    Value interface{}
    Spec  FieldSpec
}

type CompareOp int

const (
    OpEqual CompareOp = iota
    OpNotEqual
    OpGreater
    OpGreaterEqual
    OpLess
    OpLessEqual
    OpContains
)

func (p ComparisonPred) Evaluate(item interface{}) bool {
    fieldValue := p.Spec.Getter(item)

    switch p.Op {
    case OpEqual:
        return fieldValue == p.Value
    case OpNotEqual:
        return fieldValue != p.Value
    case OpGreater:
        return compare(fieldValue, p.Value) > 0
    case OpGreaterEqual:
        return compare(fieldValue, p.Value) >= 0
    case OpLess:
        return compare(fieldValue, p.Value) < 0
    case OpLessEqual:
        return compare(fieldValue, p.Value) <= 0
    case OpContains:
        return contains(fieldValue, p.Value)
    }

    return false
}

func (p ComparisonPred) String() string {
    ops := map[CompareOp]string{
        OpEqual: "=", OpNotEqual: "!=", OpGreater: ">",
        OpGreaterEqual: ">=", OpLess: "<", OpLessEqual: "<=",
        OpContains: "contains",
    }
    return fmt.Sprintf("%s %s %v", p.Field, ops[p.Op], p.Value)
}
```

**File:** `predicates/parser.go`

```go
package predicates

type Parser struct {
    fields map[string]FieldSpec
}

type FieldSpec struct {
    Name        string
    Type        FieldType
    Getter      func(item interface{}) interface{}
    Completions []string  // For enum fields
}

type FieldType int

const (
    FieldTypeBool FieldType = iota
    FieldTypeInt
    FieldTypeString
)

func NewParser() *Parser {
    return &Parser{
        fields: make(map[string]FieldSpec),
    }
}

func (p *Parser) RegisterField(spec FieldSpec) {
    p.fields[spec.Name] = spec
}

// Parse parses predicate expressions:
//   "dirty" → FieldPred("dirty")
//   "dirty and has_pr" → AndPred(FieldPred("dirty"), FieldPred("has_pr"))
//   "ahead > 5" → ComparisonPred("ahead", OpGreater, 5)
//   "dirty and (ahead > 0 or behind > 0)" → AndPred(FieldPred("dirty"), OrPred(...))
func (p *Parser) Parse(expr string) (Predicate, error) {
    tokens := p.tokenize(expr)
    return p.parseExpression(tokens)
}

func (p *Parser) tokenize(expr string) []string {
    // Simple tokenizer: split on whitespace, preserve operators
    // TODO: Handle parentheses, quoted strings
    return strings.Fields(expr)
}

func (p *Parser) parseExpression(tokens []string) (Predicate, error) {
    if len(tokens) == 0 {
        return nil, errors.New("empty expression")
    }

    return p.parseOr(tokens)
}

func (p *Parser) parseOr(tokens []string) (Predicate, error) {
    left, remaining, err := p.parseAnd(tokens)
    if err != nil {
        return nil, err
    }

    if len(remaining) > 0 && remaining[0] == "or" {
        right, remaining, err := p.parseOr(remaining[1:])
        if err != nil {
            return nil, err
        }
        return OrPred{Left: left, Right: right}, nil
    }

    return left, nil
}

func (p *Parser) parseAnd(tokens []string) (Predicate, []string, error) {
    left, remaining, err := p.parsePrimary(tokens)
    if err != nil {
        return nil, nil, err
    }

    if len(remaining) > 0 && remaining[0] == "and" {
        right, remaining, err := p.parseAnd(remaining[1:])
        if err != nil {
            return nil, nil, err
        }
        return AndPred{Left: left, Right: right}, remaining, nil
    }

    return left, remaining, nil
}

func (p *Parser) parsePrimary(tokens []string) (Predicate, []string, error) {
    if len(tokens) == 0 {
        return nil, nil, errors.New("unexpected end of expression")
    }

    token := tokens[0]

    // NOT operator
    if token == "not" {
        inner, remaining, err := p.parsePrimary(tokens[1:])
        if err != nil {
            return nil, nil, err
        }
        return NotPred{Inner: inner}, remaining, nil
    }

    // Field predicate or comparison
    spec, ok := p.fields[token]
    if !ok {
        return nil, nil, fmt.Errorf("unknown field: %s", token)
    }

    // Boolean field
    if spec.Type == FieldTypeBool {
        return FieldPred{Field: token, Spec: spec}, tokens[1:], nil
    }

    // Comparison: field OP value
    if len(tokens) < 3 {
        return nil, nil, fmt.Errorf("incomplete comparison: %s", token)
    }

    op, err := parseOp(tokens[1])
    if err != nil {
        return nil, nil, err
    }

    value, err := parseValue(tokens[2], spec.Type)
    if err != nil {
        return nil, nil, err
    }

    return ComparisonPred{
        Field: token,
        Op:    op,
        Value: value,
        Spec:  spec,
    }, tokens[3:], nil
}

func parseOp(s string) (CompareOp, error) {
    ops := map[string]CompareOp{
        "=": OpEqual, "==": OpEqual, "!=": OpNotEqual,
        ">": OpGreater, ">=": OpGreaterEqual,
        "<": OpLess, "<=": OpLessEqual,
        "contains": OpContains,
    }

    op, ok := ops[s]
    if !ok {
        return 0, fmt.Errorf("unknown operator: %s", s)
    }

    return op, nil
}

func parseValue(s string, typ FieldType) (interface{}, error) {
    switch typ {
    case FieldTypeInt:
        return strconv.Atoi(s)
    case FieldTypeString:
        return s, nil
    default:
        return nil, fmt.Errorf("cannot parse value for type: %v", typ)
    }
}
```

### 0.7 Bubble Tea Component

**File:** `component.go`

```go
package commander

import (
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

type Model struct {
    input       textinput.Model
    parser      *Parser
    ctx         CommandContext
    completions []Completion
    cursor      int
    visible     bool
    err         error
}

func New(parser *Parser, ctx CommandContext) Model {
    ti := textinput.New()
    ti.Prompt = ":"
    ti.Width = 80

    return Model{
        input:  ti,
        parser: parser,
        ctx:    ctx,
    }
}

func (m Model) IsVisible() bool {
    return m.visible
}

func (m Model) Show() Model {
    m.visible = true
    m.input.Focus()
    m.input.SetValue("")
    m.err = nil
    m.completions = nil
    m.cursor = 0
    return m
}

func (m Model) Hide() Model {
    m.visible = false
    m.input.Blur()
    return m
}

func (m Model) Value() string {
    return m.input.Value()
}

func (m Model) SetError(err error) Model {
    m.err = err
    return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
    if !m.visible {
        return m, nil
    }

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "tab":
            // Cycle through completions
            if len(m.completions) > 0 {
                m.cursor = (m.cursor + 1) % len(m.completions)
                completion := m.completions[m.cursor]
                m.input.SetValue(completion.Text)
            }
            return m, nil

        case "shift+tab":
            // Reverse cycle
            if len(m.completions) > 0 {
                m.cursor--
                if m.cursor < 0 {
                    m.cursor = len(m.completions) - 1
                }
                completion := m.completions[m.cursor]
                m.input.SetValue(completion.Text)
            }
            return m, nil

        case "enter":
            cmd, err := m.parser.Parse(m.input.Value())
            if err != nil {
                m.err = err
                return m, nil
            }

            // Execute command
            return m, func() tea.Msg {
                execErr := cmd.Execute(m.ctx)
                return CommandExecutedMsg{Cmd: cmd, Error: execErr}
            }

        case "esc":
            return m.Hide(), nil
        }
    }

    // Update input and refresh completions
    var cmd tea.Cmd
    m.input, cmd = m.input.Update(msg)

    // Get completions for current input
    m.completions = m.parser.GetCompletions(CompletionContext{
        Input:      m.input.Value(),
        CursorPos:  m.input.Position(),
        AppContext: m.ctx,
    })
    m.cursor = 0

    return m, cmd
}

func (m Model) View(width int) string {
    if !m.visible {
        return ""
    }

    var b strings.Builder

    // Command input
    inputStyle := lipgloss.NewStyle().
        Width(width).
        Padding(0, 1).
        Background(lipgloss.Color("#1e1e2e"))

    if m.err != nil {
        errorStyle := lipgloss.NewStyle().
            Foreground(lipgloss.Color("#f38ba8"))
        b.WriteString(inputStyle.Render(errorStyle.Render(m.err.Error())))
    } else {
        b.WriteString(inputStyle.Render(m.input.View()))
    }

    // Completions dropdown
    if len(m.completions) > 0 && m.err == nil {
        b.WriteString("\n")
        b.WriteString(m.renderCompletions(width))
    }

    return b.String()
}

func (m Model) renderCompletions(width int) string {
    var b strings.Builder

    maxVisible := 5
    start := 0
    if len(m.completions) > maxVisible {
        // Keep cursor in view
        if m.cursor >= maxVisible {
            start = m.cursor - maxVisible + 1
        }
    }

    end := start + maxVisible
    if end > len(m.completions) {
        end = len(m.completions)
    }

    for i := start; i < end; i++ {
        completion := m.completions[i]

        style := lipgloss.NewStyle().
            Width(width).
            Padding(0, 1)

        if i == m.cursor {
            style = style.Background(lipgloss.Color("#363a4f"))
        }

        line := fmt.Sprintf("  %-20s %s", completion.Text, completion.Description)
        b.WriteString(style.Render(line) + "\n")
    }

    return b.String()
}
```

**Deliverable:** Reusable `tui-commander` package with examples and tests.

---

## Phase 1: Command Abstraction (Week 3)

### 1.1 Define Command Interface for gh-repo-dashboard

**File:** `internal/command/command.go`

```go
package command

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/kyleking/gh-repo-dashboard/internal/models"
    "github.com/kyleking/tui-commander"
)

// Command represents a semantic action in the dashboard
// Implements tui-commander.Command interface
type Command interface {
    Execute(m *Model) (tea.Cmd, error)
    String() string
    Undo() (Command, error)
}

// Model interface for command execution
type Model interface {
    commander.CommandContext

    // State access
    GetFilteredPaths() []string
    GetMarkedRepos() []string
    GetSelectedRepo() string

    // State mutation
    SetFilter(modes []models.FilterMode)
    SetMarked(paths []string)
    SetSelected(paths []string)

    // Operations
    ExecuteBatchTask(name string, task func(path string) (bool, string)) tea.Cmd
}
```

### 1.2 Implement Core Commands

**File:** `internal/command/commands.go`

```go
package command

// Navigation commands
type NextRepoCmd struct{}
type PrevRepoCmd struct{}
type FirstRepoCmd struct{}
type LastRepoCmd struct{}
type SelectRepoCmd struct{ Index int }

// Filter commands
type SetFilterCmd struct {
    Modes []string  // ["dirty", "has_pr"]
}

type ClearFilterCmd struct{}

// Selection commands
type MarkRepoCmd struct {
    Index int
}

type UnmarkRepoCmd struct {
    Index int
}

type MarkRangeCmd struct {
    Start, End int  // Mark repos in range [start, end]
}

type ClearMarksCmd struct{}

type SelectWhereCmd struct {
    Predicate predicates.Predicate
}

// Batch operation commands
type FetchBatchCmd struct {
    MarkedOnly bool
    Predicate  predicates.Predicate
}

type PruneBatchCmd struct {
    MarkedOnly bool
    Predicate  predicates.Predicate
}

type CleanupBatchCmd struct {
    MarkedOnly bool
    Predicate  predicates.Predicate
}

// View commands
type EnterDetailCmd struct{}
type ExitDetailCmd struct{}
type NextTabCmd struct{}
type PrevTabCmd struct{}

// Search commands
type SearchCmd struct {
    Query string
}

type NextSearchResultCmd struct{}
type PrevSearchResultCmd struct{}

// Modal commands
type ShowHelpCmd struct{}
type HideHelpCmd struct{}
type ShowFilterModalCmd struct{}
type ShowSortModalCmd struct{}

// Utility commands
type RefreshCmd struct{}
type QuitCmd struct{}
```

### 1.3 Command Implementations

**File:** `internal/command/filter.go`

```go
package command

func (c SetFilterCmd) Execute(m Model) (tea.Cmd, error) {
    modes := make([]models.FilterMode, len(c.Modes))
    for i, modeStr := range c.Modes {
        mode, err := models.ParseFilterMode(modeStr)
        if err != nil {
            return nil, fmt.Errorf("invalid filter mode: %s", modeStr)
        }
        modes[i] = mode
    }

    m.SetFilter(modes)
    return nil, nil
}

func (c SetFilterCmd) String() string {
    return fmt.Sprintf("filter %s", strings.Join(c.Modes, " "))
}

func (c SetFilterCmd) Undo() (Command, error) {
    // TODO: Store previous filter state
    return ClearFilterCmd{}, nil
}
```

**File:** `internal/command/batch.go`

```go
package command

import (
    "github.com/kyleking/gh-repo-dashboard/internal/batch"
    "github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

func (c FetchBatchCmd) Execute(m Model) (tea.Cmd, error) {
    repos := c.selectRepos(m)
    if len(repos) == 0 {
        return nil, errors.New("no repos selected")
    }

    taskFunc := func(path string) (bool, string) {
        vcsOps := vcs.GetVCSOperations(path)
        return vcsOps.FetchAll(context.Background(), path)
    }

    return m.ExecuteBatchTask("Fetch", taskFunc), nil
}

func (c FetchBatchCmd) selectRepos(m Model) []string {
    if c.MarkedOnly {
        return m.GetMarkedRepos()
    }

    if c.Predicate != nil {
        // Filter by predicate
        allRepos := m.GetFilteredPaths()
        var selected []string
        for _, path := range allRepos {
            summary := m.GetRepoSummary(path)
            if c.Predicate.Evaluate(summary) {
                selected = append(selected, path)
            }
        }
        return selected
    }

    // Default: all filtered repos
    return m.GetFilteredPaths()
}

func (c FetchBatchCmd) String() string {
    if c.MarkedOnly {
        return "fetch marked"
    }
    if c.Predicate != nil {
        return fmt.Sprintf("fetch where %s", c.Predicate)
    }
    return "fetch"
}

func (c FetchBatchCmd) Undo() (Command, error) {
    return nil, errors.New("fetch cannot be undone")
}
```

**File:** `internal/command/selection.go`

```go
package command

func (c MarkRepoCmd) Execute(m Model) (tea.Cmd, error) {
    repos := m.GetFilteredPaths()
    if c.Index < 0 || c.Index >= len(repos) {
        return nil, fmt.Errorf("index out of range: %d", c.Index)
    }

    path := repos[c.Index]
    marked := m.GetMarkedRepos()

    // Toggle mark
    found := false
    for i, p := range marked {
        if p == path {
            // Already marked, remove
            marked = append(marked[:i], marked[i+1:]...)
            found = true
            break
        }
    }

    if !found {
        marked = append(marked, path)
    }

    m.SetMarked(marked)
    return nil, nil
}

func (c MarkRepoCmd) String() string {
    return fmt.Sprintf("mark %d", c.Index)
}

func (c MarkRepoCmd) Undo() (Command, error) {
    return c, nil  // Toggle is self-inverse
}

func (c SelectWhereCmd) Execute(m Model) (tea.Cmd, error) {
    allRepos := m.GetFilteredPaths()
    var selected []string

    for _, path := range allRepos {
        summary := m.GetRepoSummary(path)
        if c.Predicate.Evaluate(summary) {
            selected = append(selected, path)
        }
    }

    m.SetSelected(selected)
    return nil, nil
}

func (c SelectWhereCmd) String() string {
    return fmt.Sprintf("select where %s", c.Predicate)
}

func (c SelectWhereCmd) Undo() (Command, error) {
    return &ClearSelectionCmd{}, nil
}
```

### 1.4 Register Commands

**File:** `internal/app/registry.go`

```go
package app

import (
    "github.com/kyleking/gh-repo-dashboard/internal/command"
    "github.com/kyleking/gh-repo-dashboard/internal/models"
    "github.com/kyleking/tui-commander"
    "github.com/kyleking/tui-commander/predicates"
)

func (m *Model) createRegistry() *commander.Registry {
    registry := commander.NewRegistry()

    // Register filter command
    registry.Register(commander.CommandSpec{
        Name:        "filter",
        Aliases:     []string{"f"},
        Description: "Filter repositories by criteria",
        Category:    "filtering",
        Args: []commander.ArgSpec{
            {
                Name:        "modes",
                Type:        commander.ArgTypeStringSlice,
                Required:    true,
                Description: "Filter modes to apply",
                Completions: func(ctx commander.CommandContext, partial string) []string {
                    modes := models.AllFilterModes()
                    var suggestions []string
                    for _, mode := range modes {
                        s := mode.String()
                        if partial == "" || strings.HasPrefix(s, partial) {
                            suggestions = append(suggestions, s)
                        }
                    }
                    return suggestions
                },
            },
        },
        Factory: func(args []string) (commander.Command, error) {
            return &command.SetFilterCmd{Modes: args}, nil
        },
    })

    // Register fetch command
    registry.Register(commander.CommandSpec{
        Name:        "fetch",
        Aliases:     []string{"F"},
        Description: "Fetch repositories",
        Category:    "batch",
        Args: []commander.ArgSpec{
            {
                Name:        "target",
                Type:        commander.ArgTypeEnum,
                Required:    false,
                Description: "Target repos (all, marked, or where)",
                Completions: func(ctx commander.CommandContext, partial string) []string {
                    return []string{"all", "marked", "where"}
                },
            },
            {
                Name:        "predicate",
                Type:        commander.ArgTypePredicate,
                Required:    false,
                Description: "Predicate expression (if target is 'where')",
            },
        },
        Factory: func(args []string) (commander.Command, error) {
            if len(args) == 0 {
                return &command.FetchBatchCmd{}, nil
            }

            target := args[0]

            if target == "marked" {
                return &command.FetchBatchCmd{MarkedOnly: true}, nil
            }

            if target == "where" {
                if len(args) < 2 {
                    return nil, errors.New("fetch where: missing predicate")
                }

                predExpr := strings.Join(args[1:], " ")
                pred, err := m.predicateParser.Parse(predExpr)
                if err != nil {
                    return nil, fmt.Errorf("invalid predicate: %w", err)
                }

                return &command.FetchBatchCmd{Predicate: pred}, nil
            }

            return nil, fmt.Errorf("unknown target: %s", target)
        },
    })

    // Register mark command
    registry.Register(commander.CommandSpec{
        Name:        "mark",
        Aliases:     []string{"m"},
        Description: "Mark repositories for batch operations",
        Category:    "selection",
        Args: []commander.ArgSpec{
            {
                Name:        "indices",
                Type:        commander.ArgTypeRange,
                Required:    true,
                Description: "Repo indices (e.g., 1,3,5-7)",
            },
        },
        Factory: func(args []string) (commander.Command, error) {
            if len(args) == 0 {
                return nil, errors.New("mark: missing indices")
            }

            indices, err := parseRange(args[0])
            if err != nil {
                return nil, err
            }

            return &command.MarkRangeCmd{Indices: indices}, nil
        },
    })

    // Register select command
    registry.Register(commander.CommandSpec{
        Name:        "select",
        Aliases:     []string{"s"},
        Description: "Select repositories by predicate",
        Category:    "selection",
        Args: []commander.ArgSpec{
            {
                Name:        "where",
                Type:        commander.ArgTypeString,
                Required:    false,
                Description: "Keyword 'where' (optional)",
            },
            {
                Name:        "predicate",
                Type:        commander.ArgTypePredicate,
                Required:    true,
                Description: "Predicate expression",
            },
        },
        Factory: func(args []string) (commander.Command, error) {
            startIdx := 0
            if len(args) > 0 && args[0] == "where" {
                startIdx = 1
            }

            if len(args) <= startIdx {
                return nil, errors.New("select: missing predicate")
            }

            predExpr := strings.Join(args[startIdx:], " ")
            pred, err := m.predicateParser.Parse(predExpr)
            if err != nil {
                return nil, fmt.Errorf("invalid predicate: %w", err)
            }

            return &command.SelectWhereCmd{Predicate: pred}, nil
        },
    })

    // More commands...

    return registry
}

func parseRange(s string) ([]int, error) {
    // Parse "1,3,5-7" → [1, 3, 5, 6, 7]
    var indices []int
    parts := strings.Split(s, ",")

    for _, part := range parts {
        if strings.Contains(part, "-") {
            rangeParts := strings.Split(part, "-")
            if len(rangeParts) != 2 {
                return nil, fmt.Errorf("invalid range: %s", part)
            }

            start, err := strconv.Atoi(rangeParts[0])
            if err != nil {
                return nil, err
            }

            end, err := strconv.Atoi(rangeParts[1])
            if err != nil {
                return nil, err
            }

            for i := start; i <= end; i++ {
                indices = append(indices, i)
            }
        } else {
            idx, err := strconv.Atoi(part)
            if err != nil {
                return nil, err
            }
            indices = append(indices, idx)
        }
    }

    return indices, nil
}
```

### 1.5 Register Predicates

**File:** `internal/app/predicates.go`

```go
package app

import (
    "github.com/kyleking/gh-repo-dashboard/internal/models"
    "github.com/kyleking/tui-commander/predicates"
)

func (m *Model) createPredicateParser() *predicates.Parser {
    parser := predicates.NewParser()

    // Boolean fields
    parser.RegisterField(predicates.FieldSpec{
        Name: "dirty",
        Type: predicates.FieldTypeBool,
        Getter: func(item interface{}) interface{} {
            summary := item.(models.RepoSummary)
            return summary.Staged > 0 || summary.Unstaged > 0 || summary.Untracked > 0
        },
    })

    parser.RegisterField(predicates.FieldSpec{
        Name: "has_pr",
        Type: predicates.FieldTypeBool,
        Getter: func(item interface{}) interface{} {
            summary := item.(models.RepoSummary)
            return summary.PRInfo != nil
        },
    })

    parser.RegisterField(predicates.FieldSpec{
        Name: "has_stash",
        Type: predicates.FieldTypeBool,
        Getter: func(item interface{}) interface{} {
            summary := item.(models.RepoSummary)
            return summary.Stashes > 0
        },
    })

    // Integer fields
    parser.RegisterField(predicates.FieldSpec{
        Name: "ahead",
        Type: predicates.FieldTypeInt,
        Getter: func(item interface{}) interface{} {
            summary := item.(models.RepoSummary)
            return summary.Ahead
        },
    })

    parser.RegisterField(predicates.FieldSpec{
        Name: "behind",
        Type: predicates.FieldTypeInt,
        Getter: func(item interface{}) interface{} {
            summary := item.(models.RepoSummary)
            return summary.Behind
        },
    })

    parser.RegisterField(predicates.FieldSpec{
        Name: "staged",
        Type: predicates.FieldTypeInt,
        Getter: func(item interface{}) interface{} {
            summary := item.(models.RepoSummary)
            return summary.Staged
        },
    })

    // String fields
    parser.RegisterField(predicates.FieldSpec{
        Name: "branch",
        Type: predicates.FieldTypeString,
        Getter: func(item interface{}) interface{} {
            summary := item.(models.RepoSummary)
            return summary.Branch
        },
    })

    parser.RegisterField(predicates.FieldSpec{
        Name: "vcs",
        Type: predicates.FieldTypeString,
        Getter: func(item interface{}) interface{} {
            summary := item.(models.RepoSummary)
            return summary.VCSType.String()
        },
        Completions: []string{"git", "jj"},
    })

    return parser
}
```

### 1.6 Integrate Command Mode

**File:** `internal/app/app.go`

```go
type Model struct {
    // ... existing fields

    // Command mode
    cmdMode         commander.Model
    cmdRegistry     *commander.Registry
    predicateParser *predicates.Parser
    commandLog      []command.Command

    // Selection state
    markedRepos   []string
    selectedRepos []string  // Repos selected by predicate
}

func New(scanPaths []string, maxDepth int) Model {
    m := Model{
        // ... existing initialization
    }

    // Setup command system
    m.predicateParser = m.createPredicateParser()
    m.cmdRegistry = m.createRegistry()
    parser := commander.NewParser(m.cmdRegistry)
    m.cmdMode = commander.New(parser, &m)

    return m
}

// Implement CommandContext interface
func (m *Model) GetState() interface{} {
    return m.toAppState()
}

func (m *Model) SetState(state interface{}) error {
    appState := state.(AppState)
    m.fromAppState(appState)
    return nil
}
```

**File:** `internal/app/update.go`

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Handle command mode
    if m.cmdMode.IsVisible() {
        return m.handleCommandMode(msg)
    }

    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Enter command mode
        if msg.String() == ":" {
            m.cmdMode = m.cmdMode.Show()
            return m, nil
        }

        // Existing key handling
        return m.handleKey(msg)

    case commander.CommandExecutedMsg:
        if msg.Error != nil {
            m.statusMessage = fmt.Sprintf("Error: %s", msg.Error)
        } else {
            m.statusMessage = fmt.Sprintf("Executed: %s", msg.Cmd.String())
            m.commandLog = append(m.commandLog, msg.Cmd)
        }
        return m, nil

    // ... other messages
    }

    return m, nil
}

func (m Model) handleCommandMode(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    m.cmdMode, cmd = m.cmdMode.Update(msg)
    return m, cmd
}
```

**Deliverable:** Command abstraction integrated. Commands can be sent directly or via command mode.

---

## Phase 2: Serializable State (Week 4)

### 2.1 Define AppState

**File:** `internal/state/appstate.go`

```go
package state

// AppState contains all serializable application state
type AppState struct {
    // View state
    ViewMode     string `json:"view_mode"`
    SelectedRepo string `json:"selected_repo"`
    Cursor       int    `json:"cursor"`

    // Filtering
    ActiveFilters []string `json:"active_filters"`
    SearchText    string   `json:"search_text"`

    // Selection
    MarkedRepos   []string `json:"marked_repos"`
    SelectedRepos []string `json:"selected_repos"`

    // Detail view
    DetailTab    string `json:"detail_tab"`
    DetailCursor int    `json:"detail_cursor"`

    // Sort
    ActiveSorts []SortSpec `json:"active_sorts"`
}

type SortSpec struct {
    Mode      string `json:"mode"`
    Direction string `json:"direction"`
    Priority  int    `json:"priority"`
}

func (a *AppState) ToJSON() ([]byte, error) {
    return json.MarshalIndent(a, "", "  ")
}

func (a *AppState) FromJSON(data []byte) error {
    return json.Unmarshal(data, a)
}
```

### 2.2 Separate UIState

**File:** `internal/state/uistate.go`

```go
package state

import (
    "github.com/charmbracelet/bubbles/help"
    "github.com/charmbracelet/bubbles/textinput"
)

// UIState contains ephemeral UI state (not serialized)
type UIState struct {
    Width  int
    Height int

    // Modals and inputs
    SearchInput textinput.Model
    Help        help.Model

    // Loading indicators
    Loading      bool
    LoadingCount int
    LoadedCount  int

    // Cache state (not serialized)
    CacheHits   int
    CacheMisses int
}
```

### 2.3 Refactor Model

**File:** `internal/app/app.go`

```go
type Model struct {
    App state.AppState
    UI  state.UIState

    // Reference data (not part of state)
    scanPaths []string
    maxDepth  int
    repoPaths []string
    summaries map[string]models.RepoSummary

    // ... rest
}

func (m Model) toAppState() state.AppState {
    return m.App
}

func (m *Model) fromAppState(s state.AppState) {
    m.App = s
}

func (m Model) SaveSnapshot(path string) error {
    data, err := m.App.ToJSON()
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0644)
}

func (m *Model) LoadSnapshot(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }

    var appState state.AppState
    if err := appState.FromJSON(data); err != nil {
        return err
    }

    m.fromAppState(appState)
    return nil
}
```

**Deliverable:** State is serializable. Can snapshot/restore for testing.

---

## Phase 3: Text Objects + Operators (Week 5-6)

### 3.1 Define Text Objects

**File:** `internal/textobjects/textobjects.go`

```go
package textobjects

import (
    "github.com/kyleking/gh-repo-dashboard/internal/models"
    "github.com/kyleking/tui-commander/predicates"
)

// TextObject represents a selector for repositories
type TextObject interface {
    Select(repos []models.RepoSummary) []models.RepoSummary
    String() string
}

// DirtyRepos: dr
type DirtyRepos struct{}

func (t DirtyRepos) Select(repos []models.RepoSummary) []models.RepoSummary {
    var selected []models.RepoSummary
    for _, r := range repos {
        if r.Staged > 0 || r.Unstaged > 0 || r.Untracked > 0 {
            selected = append(selected, r)
        }
    }
    return selected
}

func (t DirtyRepos) String() string { return "dr" }

// PRRepos: pr
type PRRepos struct{}

func (t PRRepos) Select(repos []models.RepoSummary) []models.RepoSummary {
    var selected []models.RepoSummary
    for _, r := range repos {
        if r.PRInfo != nil {
            selected = append(selected, r)
        }
    }
    return selected
}

func (t PRRepos) String() string { return "pr" }

// AllRepos: ar
type AllRepos struct{}

func (t AllRepos) Select(repos []models.RepoSummary) []models.RepoSummary {
    return repos
}

func (t AllRepos) String() string { return "ar" }

// BehindRepos: br
type BehindRepos struct{}

func (t BehindRepos) Select(repos []models.RepoSummary) []models.RepoSummary {
    var selected []models.RepoSummary
    for _, r := range repos {
        if r.Behind > 0 {
            selected = append(selected, r)
        }
    }
    return selected
}

func (t BehindRepos) String() string { return "br" }

// AheadRepos: ar (conflicts with AllRepos)
// Use 'hr' (ahead repos) instead
type AheadRepos struct{}

func (t AheadRepos) Select(repos []models.RepoSummary) []models.RepoSummary {
    var selected []models.RepoSummary
    for _, r := range repos {
        if r.Ahead > 0 {
            selected = append(selected, r)
        }
    }
    return selected
}

func (t AheadRepos) String() string { return "hr" }

// StashedRepos: sr
type StashedRepos struct{}

func (t StashedRepos) Select(repos []models.RepoSummary) []models.RepoSummary {
    var selected []models.RepoSummary
    for _, r := range repos {
        if r.Stashes > 0 {
            selected = append(selected, r)
        }
    }
    return selected
}

func (t StashedRepos) String() string { return "sr" }
```

### 3.2 Define Operators

**File:** `internal/operators/operators.go`

```go
package operators

import (
    "github.com/kyleking/gh-repo-dashboard/internal/command"
    "github.com/kyleking/gh-repo-dashboard/internal/textobjects"
)

// Operator represents an action on repositories
type Operator interface {
    Apply(textObj textobjects.TextObject) command.Command
    String() string
}

// FetchOperator: F
type FetchOperator struct{}

func (o FetchOperator) Apply(textObj textobjects.TextObject) command.Command {
    return &command.FetchWithTextObjectCmd{TextObject: textObj}
}

func (o FetchOperator) String() string { return "F" }

// PruneOperator: P
type PruneOperator struct{}

func (o PruneOperator) Apply(textObj textobjects.TextObject) command.Command {
    return &command.PruneWithTextObjectCmd{TextObject: textObj}
}

func (o PruneOperator) String() string { return "P" }

// CleanupOperator: C
type CleanupOperator struct{}

func (o CleanupOperator) Apply(textObj textobjects.TextObject) command.Command {
    return &command.CleanupWithTextObjectCmd{TextObject: textObj}
}

func (o CleanupOperator) String() string { return "C" }

// DeleteOperator: d (hide from view)
type DeleteOperator struct{}

func (o DeleteOperator) Apply(textObj textobjects.TextObject) command.Command {
    return &command.HideReposCmd{TextObject: textObj}
}

func (o DeleteOperator) String() string { return "d" }
```

### 3.3 Operator × Text Object Commands

**File:** `internal/command/composition.go`

```go
package command

import (
    "github.com/kyleking/gh-repo-dashboard/internal/textobjects"
)

type FetchWithTextObjectCmd struct {
    TextObject textobjects.TextObject
}

func (c FetchWithTextObjectCmd) Execute(m Model) (tea.Cmd, error) {
    allRepos := m.GetAllRepoSummaries()
    selected := c.TextObject.Select(allRepos)

    paths := make([]string, len(selected))
    for i, repo := range selected {
        paths[i] = repo.Path
    }

    if len(paths) == 0 {
        return nil, errors.New("no repos matched text object")
    }

    taskFunc := func(path string) (bool, string) {
        vcsOps := vcs.GetVCSOperations(path)
        return vcsOps.FetchAll(context.Background(), path)
    }

    return m.ExecuteBatchTask("Fetch", taskFunc, paths), nil
}

func (c FetchWithTextObjectCmd) String() string {
    return fmt.Sprintf("fetch %s", c.TextObject.String())
}

func (c FetchWithTextObjectCmd) Undo() (Command, error) {
    return nil, errors.New("fetch cannot be undone")
}
```

### 3.4 Keybinding Layer for Operators

**File:** `internal/app/operators.go`

```go
package app

import (
    "github.com/kyleking/gh-repo-dashboard/internal/operators"
    "github.com/kyleking/gh-repo-dashboard/internal/textobjects"
)

// OperatorMode tracks pending operator waiting for text object
type OperatorMode struct {
    Active   bool
    Operator operators.Operator
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    key := msg.String()

    // Handle operator mode (waiting for text object)
    if m.operatorMode.Active {
        return m.handleTextObject(key)
    }

    // Check for operator keys
    switch key {
    case "F":
        if m.viewMode == ViewModeRepoList {
            m.operatorMode = OperatorMode{
                Active:   true,
                Operator: operators.FetchOperator{},
            }
            m.statusMessage = "Fetch: select text object (dr/pr/ar/...)"
            return m, nil
        }

    case "P":
        if m.viewMode == ViewModeRepoList {
            m.operatorMode = OperatorMode{
                Active:   true,
                Operator: operators.PruneOperator{},
            }
            m.statusMessage = "Prune: select text object (dr/pr/ar/...)"
            return m, nil
        }

    case "C":
        if m.viewMode == ViewModeRepoList {
            m.operatorMode = OperatorMode{
                Active:   true,
                Operator: operators.CleanupOperator{},
            }
            m.statusMessage = "Cleanup: select text object (dr/pr/ar/...)"
            return m, nil
        }

    case "d":
        if m.viewMode == ViewModeRepoList {
            m.operatorMode = OperatorMode{
                Active:   true,
                Operator: operators.DeleteOperator{},
            }
            m.statusMessage = "Hide: select text object (dr/pr/ar/...)"
            return m, nil
        }
    }

    // ... rest of key handling
}

func (m Model) handleTextObject(key string) (tea.Model, tea.Cmd) {
    var textObj textobjects.TextObject

    // Parse two-character text objects
    switch key {
    case "dr":
        textObj = textobjects.DirtyRepos{}
    case "pr":
        textObj = textobjects.PRRepos{}
    case "ar":
        textObj = textobjects.AllRepos{}
    case "br":
        textObj = textobjects.BehindRepos{}
    case "hr":
        textObj = textobjects.AheadRepos{}
    case "sr":
        textObj = textobjects.StashedRepos{}
    default:
        // Not a valid text object, cancel operator mode
        m.operatorMode = OperatorMode{Active: false}
        m.statusMessage = fmt.Sprintf("Unknown text object: %s", key)
        return m, nil
    }

    // Apply operator to text object
    cmd := m.operatorMode.Operator.Apply(textObj)

    // Reset operator mode
    m.operatorMode = OperatorMode{Active: false}

    // Execute command
    teaCmd, err := cmd.Execute(&m)
    if err != nil {
        m.statusMessage = fmt.Sprintf("Error: %s", err)
        return m, nil
    }

    m.commandLog = append(m.commandLog, cmd)
    return m, teaCmd
}
```

**Note**: Two-character text objects require special handling. We need to accumulate keypresses:

```go
type Model struct {
    // ... existing fields

    operatorMode     OperatorMode
    pendingKeys      string  // Accumulate keypresses for text objects
    pendingKeysTimer *time.Timer
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    key := msg.String()

    // In operator mode, accumulate keys for text object
    if m.operatorMode.Active {
        m.pendingKeys += key

        // Try to match text object
        if textObj := m.matchTextObject(m.pendingKeys); textObj != nil {
            cmd := m.operatorMode.Operator.Apply(textObj)
            m.operatorMode = OperatorMode{Active: false}
            m.pendingKeys = ""

            teaCmd, err := cmd.Execute(&m)
            if err != nil {
                m.statusMessage = fmt.Sprintf("Error: %s", err)
                return m, nil
            }

            return m, teaCmd
        }

        // If two keys and no match, cancel
        if len(m.pendingKeys) >= 2 {
            m.operatorMode = OperatorMode{Active: false}
            m.pendingKeys = ""
            m.statusMessage = fmt.Sprintf("Unknown text object: %s", m.pendingKeys)
        }

        return m, nil
    }

    // ... operator key handling
}

func (m Model) matchTextObject(keys string) textobjects.TextObject {
    switch keys {
    case "dr":
        return textobjects.DirtyRepos{}
    case "pr":
        return textobjects.PRRepos{}
    case "ar":
        return textobjects.AllRepos{}
    case "br":
        return textobjects.BehindRepos{}
    case "hr":
        return textobjects.AheadRepos{}
    case "sr":
        return textobjects.StashedRepos{}
    default:
        return nil
    }
}
```

**Deliverable:** Full vim-paradigm operators × text objects working. `Fdr` fetches dirty repos.

---

## Phase 4: Fixture-Based Testing (Week 7)

### 4.1 Define Fixture Format

**File:** `internal/fixtures/schema.go`

```go
package fixtures

import (
    "github.com/kyleking/gh-repo-dashboard/internal/command"
    "github.com/kyleking/gh-repo-dashboard/internal/state"
)

type Fixture struct {
    Name     string              `yaml:"name"`
    Before   FixtureState        `yaml:"before"`
    Commands []CommandSpec       `yaml:"commands"`
    After    FixtureState        `yaml:"after"`
    Expect   ExpectedOutcome     `yaml:"expect"`
}

type FixtureState struct {
    AppState state.AppState      `yaml:"app_state"`
    Repos    []RepoSpec          `yaml:"repos"`
}

type RepoSpec struct {
    Path      string `yaml:"path"`
    Branch    string `yaml:"branch"`
    VCSType   string `yaml:"vcs_type"`
    Dirty     bool   `yaml:"dirty"`
    Ahead     int    `yaml:"ahead"`
    Behind    int    `yaml:"behind"`
    HasPR     bool   `yaml:"has_pr"`
    Stashes   int    `yaml:"stashes"`
}

type CommandSpec struct {
    Type   string                 `yaml:"type"`
    Params map[string]interface{} `yaml:"params,omitempty"`
}

type ExpectedOutcome struct {
    VCSCommands []string `yaml:"vcs_commands,omitempty"`
    Error       string   `yaml:"error,omitempty"`
    StatusMsg   string   `yaml:"status_msg,omitempty"`
}

func LoadFixtures(path string) ([]Fixture, error)
func (f *Fixture) ToCommands() ([]command.Command, error)
```

### 4.2 Example Fixtures

**File:** `internal/fixtures/filter_and_batch.yaml`

```yaml
- name: "Filter to dirty repos and fetch"
  before:
    repos:
      - path: "/proj/api"
        branch: "main"
        vcs_type: "git"
        dirty: true
        ahead: 2
      - path: "/proj/web"
        branch: "feature/auth"
        vcs_type: "git"
        dirty: false
        has_pr: true
      - path: "/proj/lib"
        branch: "develop"
        vcs_type: "jj"
        dirty: true
  commands:
    - type: "set_filter"
      params:
        modes: ["dirty"]
    - type: "fetch_batch"
  after:
    app_state:
      active_filters: ["dirty"]
  expect:
    vcs_commands:
      - "cd /proj/api && git fetch --all --prune"
      - "cd /proj/lib && jj git fetch --all-remotes"

- name: "Fetch dirty repos using operator × text object"
  before:
    repos:
      - path: "/proj/api"
        dirty: true
      - path: "/proj/web"
        dirty: false
      - path: "/proj/lib"
        dirty: true
  commands:
    - type: "fetch_with_text_object"
      params:
        text_object: "dr"
  expect:
    vcs_commands:
      - "cd /proj/api && git fetch --all --prune"
      - "cd /proj/lib && git fetch --all --prune"

- name: "Select repos by predicate and cleanup"
  before:
    repos:
      - path: "/proj/api"
        dirty: true
        has_pr: true
      - path: "/proj/web"
        dirty: false
        has_pr: true
      - path: "/proj/lib"
        dirty: true
        has_pr: false
  commands:
    - type: "select_where"
      params:
        predicate: "dirty and has_pr"
    - type: "cleanup_batch"
  after:
    app_state:
      selected_repos: ["/proj/api"]
  expect:
    vcs_commands:
      - "cd /proj/api && git branch --merged main | grep -v main | xargs git branch -d"

- name: "Mark specific repos and fetch"
  before:
    repos:
      - path: "/proj/api"
      - path: "/proj/web"
      - path: "/proj/lib"
      - path: "/proj/db"
  commands:
    - type: "mark_repo"
      params:
        index: 0
    - type: "mark_repo"
      params:
        index: 2
    - type: "fetch_batch"
      params:
        marked_only: true
  after:
    app_state:
      marked_repos: ["/proj/api", "/proj/lib"]
  expect:
    vcs_commands:
      - "cd /proj/api && git fetch --all --prune"
      - "cd /proj/lib && git fetch --all --prune"
```

**File:** `internal/fixtures/navigation.yaml`

```yaml
- name: "Navigate to next repo"
  before:
    app_state:
      cursor: 0
    repos:
      - path: "/proj/api"
      - path: "/proj/web"
  commands:
    - type: "next_repo"
  after:
    app_state:
      cursor: 1

- name: "Navigate to last repo"
  before:
    app_state:
      cursor: 0
    repos:
      - path: "/proj/api"
      - path: "/proj/web"
      - path: "/proj/lib"
  commands:
    - type: "last_repo"
  after:
    app_state:
      cursor: 2
```

**File:** `internal/fixtures/command_mode.yaml`

```yaml
- name: "Command mode: filter dirty has_pr"
  before:
    repos:
      - path: "/proj/api"
        dirty: true
        has_pr: true
      - path: "/proj/web"
        dirty: false
        has_pr: true
      - path: "/proj/lib"
        dirty: true
        has_pr: false
  commands:
    - type: "parse_command"
      params:
        input: "filter dirty has_pr"
  after:
    app_state:
      active_filters: ["dirty", "has_pr"]

- name: "Command mode: mark 1,3 then fetch marked"
  before:
    repos:
      - path: "/proj/api"
      - path: "/proj/web"
      - path: "/proj/lib"
      - path: "/proj/db"
  commands:
    - type: "parse_command"
      params:
        input: "mark 1,3"
    - type: "parse_command"
      params:
        input: "fetch marked"
  after:
    app_state:
      marked_repos: ["/proj/web", "/proj/db"]
  expect:
    vcs_commands:
      - "cd /proj/web && git fetch --all --prune"
      - "cd /proj/db && git fetch --all --prune"
```

### 4.3 Test Harness

**File:** `internal/fixtures/harness_test.go`

```go
package fixtures_test

import (
    "testing"

    "github.com/kyleking/gh-repo-dashboard/internal/fixtures"
    "github.com/kyleking/gh-repo-dashboard/internal/app"
    "github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

func TestFixtures(t *testing.T) {
    fixtureFiles := []string{
        "filter_and_batch.yaml",
        "navigation.yaml",
        "command_mode.yaml",
        "text_objects.yaml",
        "predicates.yaml",
    }

    for _, file := range fixtureFiles {
        fixtures, err := fixtures.LoadFixtures(file)
        if err != nil {
            t.Fatalf("Failed to load %s: %v", file, err)
        }

        for _, fx := range fixtures {
            t.Run(fx.Name, func(t *testing.T) {
                runFixture(t, fx)
            })
        }
    }
}

func runFixture(t *testing.T, fx fixtures.Fixture) {
    // Create mock VCS operations
    mockVCS := vcs.NewMockVCS()

    // Create test model
    m := app.New(nil, 0)

    // Load fixture state
    m = m.FromFixtureState(fx.Before)

    // Setup mock repos
    for _, repoSpec := range fx.Before.Repos {
        summary := repoSpec.ToRepoSummary()
        m.SetRepoSummary(summary)
        mockVCS.RegisterRepo(summary.Path)
    }

    // Execute commands
    commands, err := fx.ToCommands()
    if err != nil {
        t.Fatalf("Failed to convert commands: %v", err)
    }

    for i, cmd := range commands {
        _, err := cmd.Execute(&m)
        if fx.Expect.Error != "" {
            if err == nil {
                t.Fatalf("Command %d expected error, got none", i)
            }
            if err.Error() != fx.Expect.Error {
                t.Errorf("Command %d error mismatch:\nGot:  %s\nWant: %s",
                    i, err.Error(), fx.Expect.Error)
            }
            return
        }
        if err != nil {
            t.Fatalf("Command %d failed: %v", i, err)
        }
    }

    // Verify final state
    actual := m.ToAppState()
    expected := fx.After.AppState

    if !statesEqual(actual, expected) {
        t.Errorf("State mismatch:\nGot:  %+v\nWant: %+v", actual, expected)
    }

    // Verify VCS commands
    if len(fx.Expect.VCSCommands) > 0 {
        actualCmds := mockVCS.GetExecutedCommands()
        if !commandsEqual(actualCmds, fx.Expect.VCSCommands) {
            t.Errorf("VCS commands mismatch:\nGot:  %v\nWant: %v",
                actualCmds, fx.Expect.VCSCommands)
        }
    }
}
```

### 4.4 Documentation Generator

**File:** `internal/fixtures/docgen.go`

```go
package fixtures

import (
    "fmt"
    "strings"
)

type DocGenerator struct {
    fixtures []Fixture
}

func NewDocGenerator(fixtures []Fixture) *DocGenerator {
    return &DocGenerator{fixtures: fixtures}
}

func (g *DocGenerator) GenerateMarkdown() string {
    var out strings.Builder

    out.WriteString("# gh-repo-dashboard Usage Guide\n\n")
    out.WriteString("Auto-generated from test fixtures.\n\n")

    // Group by category
    categories := map[string][]Fixture{
        "Navigation":    {},
        "Filtering":     {},
        "Batch Operations": {},
        "Text Objects":  {},
        "Command Mode":  {},
        "Predicates":    {},
    }

    for _, fx := range g.fixtures {
        cat := g.categorize(fx)
        categories[cat] = append(categories[cat], fx)
    }

    // Render by category
    for _, catName := range []string{"Navigation", "Filtering", "Batch Operations", "Text Objects", "Command Mode", "Predicates"} {
        fixtures := categories[catName]
        if len(fixtures) == 0 {
            continue
        }

        out.WriteString(fmt.Sprintf("## %s\n\n", catName))

        for _, fx := range fixtures {
            g.writeFixture(&out, fx)
        }
    }

    return out.String()
}

func (g *DocGenerator) writeFixture(out *strings.Builder, fx Fixture) {
    out.WriteString(fmt.Sprintf("### %s\n\n", fx.Name))

    // Command sequence
    out.WriteString("**Actions:**\n")
    for i, cmd := range fx.Commands {
        cmdStr := g.commandToString(cmd)
        out.WriteString(fmt.Sprintf("%d. %s\n", i+1, cmdStr))
    }
    out.WriteString("\n")

    // State changes
    changes := g.diffStates(fx.Before.AppState, fx.After.AppState)
    if len(changes) > 0 {
        out.WriteString("**Changes:**\n")
        for _, change := range changes {
            out.WriteString(fmt.Sprintf("- %s: `%v` → `%v`\n",
                change.Field, change.Before, change.After))
        }
        out.WriteString("\n")
    }

    // Expected outcome
    if len(fx.Expect.VCSCommands) > 0 {
        out.WriteString("**VCS Commands:**\n```bash\n")
        for _, cmd := range fx.Expect.VCSCommands {
            out.WriteString(cmd + "\n")
        }
        out.WriteString("```\n\n")
    }

    // Keyboard shortcuts
    keys := g.commandsToKeys(fx.Commands)
    if len(keys) > 0 {
        out.WriteString("**Keyboard:** ")
        out.WriteString(strings.Join(keys, " → "))
        out.WriteString("\n\n")
    }

    // Command mode equivalent
    cmdMode := g.commandsToCommandMode(fx.Commands)
    if len(cmdMode) > 0 {
        out.WriteString("**Command mode:** ")
        for i, cmd := range cmdMode {
            if i > 0 {
                out.WriteString(" → ")
            }
            out.WriteString(fmt.Sprintf("`%s`", cmd))
        }
        out.WriteString("\n\n")
    }

    out.WriteString("---\n\n")
}

func (g *DocGenerator) commandToString(cmd CommandSpec) string {
    // Convert command spec to human-readable string
}

func (g *DocGenerator) categorize(fx Fixture) string {
    // Categorize fixture based on command types
}

func (g *DocGenerator) commandsToKeys(commands []CommandSpec) []string {
    // Map commands to keyboard shortcuts
}

func (g *DocGenerator) commandsToCommandMode(commands []CommandSpec) []string {
    // Map commands to command mode syntax
}
```

**Deliverable:** Comprehensive fixture coverage (50-70 fixtures) with auto-generated documentation.

---

## Implementation Timeline

| Week | Phase | Deliverable |
|------|-------|-------------|
| 1-2 | Phase 0 | `tui-commander` package with examples |
| 3 | Phase 1.1-1.4 | Command abstraction, registry |
| 3 | Phase 1.5-1.6 | Predicates, command mode integration |
| 4 | Phase 2 | Serializable state (AppState/UIState) |
| 5 | Phase 3.1-3.2 | Text objects, operators |
| 5-6 | Phase 3.3-3.4 | Operator × text object composition |
| 7 | Phase 4 | Fixture-based testing + docs |

**Total: 7 weeks**

---

## Success Metrics

**Composability:**
- `Fdr` (fetch dirty) works
- `Cpr` (cleanup PRs) works
- Text objects compose: `F` + `dr` = fetch dirty repos

**Testability:**
- 50-70 fixtures covering all workflows
- No keyboard simulation in tests
- State snapshots work

**Documentation:**
- Auto-generated from fixtures
- Zero stale examples
- Both keyboard and command mode documented

**Reusability:**
- `tui-commander` used in jj-diff
- Predicate system reusable
- Command pattern reusable

---

## Migration Strategy

**Week 1-2**: Build `tui-commander` in isolation
**Week 3**: Add commands alongside existing key handlers (both work)
**Week 4**: Serializable state, start using snapshots
**Week 5-6**: Add operators/text objects as new features
**Week 7**: Add fixtures, compare with existing tests

**Rollback**: Each phase is isolated. Can stop at any phase and keep benefits.

---

## Future Enhancements

**Command recording:**
```bash
:record session.cmd     # Start recording
# ... perform operations
:stop                   # Stop recording
:replay session.cmd     # Replay commands
```

**Macro support:**
```
:record a               # Record to register 'a'
# ... operations
:stop
@a                      # Replay macro 'a'
```

**Script execution:**
```bash
gh repo-dashboard --script commands.txt
# commands.txt:
# filter dirty
# mark 1,3,5
# fetch marked
```

**Integration with gh CLI:**
```bash
gh repo fetch-dirty ~/Developer    # Uses text objects
gh repo cleanup-merged ~/Developer  # Uses operators
```
