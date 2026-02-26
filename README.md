# funcorder-fix

**[English](#english) · [Русский](#русский)**

---

<a name="english"></a>

## funcorder-fix

An automatic fixer for the [`funcorder`](https://github.com/manuelarte/funcorder) linter. The original linter detects method ordering violations in Go code but provides no way to fix them automatically. `funcorder-fix` closes that gap.

### What it does

`funcorder` enforces two rules about method ordering within a struct:

1. **Constructors first** — functions named `New*`, `Must*`, or `Or*` must appear before other methods
2. **Exported before unexported** — public methods must appear before private methods

`funcorder-fix` detects these violations and rewrites the source file with methods in the correct order, preserving all comments (doc comments, inline comments, floating comments) and all non-method content (standalone functions, constants, blank lines) exactly as written.

### Installation

```bash
go install github.com/vajrock/funcorder-fix/cmd/funcorder-fix@latest
```

Or build from source:

```bash
git clone https://github.com/vajrock/funcorder-fix.git
cd funcorder-fix
make build          # → bin/funcorder-fix
```

**Requirements:** Go 1.23+

### Usage

```bash
# Check for violations (no changes)
funcorder-fix ./...

# Show violations with details
funcorder-fix -v ./...

# Fix and print result to stdout
funcorder-fix --fix ./...

# Fix and write back to files (in-place)
funcorder-fix --fix -w ./...

# Fix a single file, write back
funcorder-fix --fix -w ./internal/service.go

# Show what would change (diff mode)
funcorder-fix --fix -d ./...
```

### Flags

| Flag | Description |
|------|-------------|
| `--fix` | Apply automatic fixes |
| `-w` | Write fixed content back to source files |
| `-d` | Print a diff instead of rewriting |
| `-l` | List only the files that have violations |
| `-v` | Verbose output (printed to stderr) |
| `--no-constructor` | Skip the constructor ordering check |
| `--no-exported` | Skip the exported/unexported ordering check |

### Before / After example

**Before** (`-v` reports 16 violations):
```go
type UserService struct { ... }

func (s *UserService) getByID(ctx context.Context, id int) (*User, error) { ... }
func NewUserService(repo Repository) *UserService { ... }
func (s *UserService) Create(ctx context.Context, u *User) error { ... }
func (s *UserService) Delete(ctx context.Context, id int) error { ... }
func (s *UserService) validate(u *User) error { ... }
```

**After** (`funcorder-fix --fix -w`):
```go
type UserService struct { ... }

func NewUserService(repo Repository) *UserService { ... }
func (s *UserService) Create(ctx context.Context, u *User) error { ... }
func (s *UserService) Delete(ctx context.Context, id int) error { ... }
func (s *UserService) getByID(ctx context.Context, id int) (*User, error) { ... }
func (s *UserService) validate(u *User) error { ... }
```

### golangci-lint integration

Until `funcorder` gains native fix support, you can add `funcorder-fix` as a pre-commit step or CI job alongside `golangci-lint`:

```bash
# Fix first, then lint
funcorder-fix --fix -w ./...
golangci-lint run -E funcorder ./...
```

### How it works

The fixer avoids the Go AST printer entirely. Instead, it works directly on the raw source bytes:

1. Parse the file to detect which structs need reordering and in what order
2. For each method, extract its exact byte range (including its doc comment)
3. Replace each method's byte slot with the text of the method that belongs there
4. Gaps between slots — standalone helper functions, blank lines — are never touched

This approach guarantees that all comments and formatting survive reordering unchanged.

---

<a name="русский"></a>

## funcorder-fix (Русский)

Автоматический исправитель для линтера [`funcorder`](https://github.com/manuelarte/funcorder). Оригинальный линтер обнаруживает нарушения порядка методов в Go-коде, но не умеет их исправлять. `funcorder-fix` решает эту проблему.

### Что делает

`funcorder` проверяет два правила упорядочивания методов структуры:

1. **Конструкторы перед остальными** — функции с именами `New*`, `Must*` или `Or*` должны стоять перед другими методами
2. **Экспортированные перед неэкспортированными** — публичные методы должны идти раньше приватных

`funcorder-fix` находит нарушения и переписывает исходный файл с методами в правильном порядке, сохраняя все комментарии (doc-комментарии, встроенные, плавающие) и весь код, не относящийся к методам (отдельные функции, константы, пустые строки), в неизменном виде.

### Установка

```bash
go install github.com/vajrock/funcorder-fix/cmd/funcorder-fix@latest
```

Или сборка из исходников:

```bash
git clone https://github.com/vajrock/funcorder-fix.git
cd funcorder-fix
make build          # → bin/funcorder-fix
```

**Требования:** Go 1.23+

### Использование

```bash
# Проверить нарушения (без изменений)
funcorder-fix ./...

# Показать нарушения с деталями
funcorder-fix -v ./...

# Исправить и вывести результат в stdout
funcorder-fix --fix ./...

# Исправить и записать обратно в файлы (in-place)
funcorder-fix --fix -w ./...

# Исправить один файл, записать обратно
funcorder-fix --fix -w ./internal/service.go

# Показать что изменится (режим diff)
funcorder-fix --fix -d ./...
```

### Флаги

| Флаг | Описание |
|------|----------|
| `--fix` | Применить автоматические исправления |
| `-w` | Записать исправленный код обратно в файлы |
| `-d` | Показать diff вместо перезаписи |
| `-l` | Перечислить только файлы с нарушениями |
| `-v` | Подробный вывод (в stderr) |
| `--no-constructor` | Отключить проверку порядка конструкторов |
| `--no-exported` | Отключить проверку экспортированных/неэкспортированных |

### Пример до / после

**До** (`-v` сообщает о 16 нарушениях):
```go
type UserService struct { ... }

func (s *UserService) getByID(ctx context.Context, id int) (*User, error) { ... }
func NewUserService(repo Repository) *UserService { ... }
func (s *UserService) Create(ctx context.Context, u *User) error { ... }
func (s *UserService) Delete(ctx context.Context, id int) error { ... }
func (s *UserService) validate(u *User) error { ... }
```

**После** (`funcorder-fix --fix -w`):
```go
type UserService struct { ... }

func NewUserService(repo Repository) *UserService { ... }
func (s *UserService) Create(ctx context.Context, u *User) error { ... }
func (s *UserService) Delete(ctx context.Context, id int) error { ... }
func (s *UserService) getByID(ctx context.Context, id int) (*User, error) { ... }
func (s *UserService) validate(u *User) error { ... }
```

### Интеграция с golangci-lint

До появления встроенной поддержки фиксов в `funcorder`, можно добавить `funcorder-fix` как шаг pre-commit или задачу CI рядом с `golangci-lint`:

```bash
# Сначала исправить, затем проверить линтером
funcorder-fix --fix -w ./...
golangci-lint run -E funcorder ./...
```

### Как работает

Инструмент не использует AST-принтер Go. Вместо этого он работает напрямую с байтами исходного кода:

1. Парсит файл, чтобы определить, какие структуры требуют переупорядочивания и в каком порядке
2. Для каждого метода извлекает точный диапазон байт (включая doc-комментарий)
3. Заменяет байтовый слот каждого метода текстом того метода, который должен быть на этом месте
4. Промежутки между слотами — отдельные вспомогательные функции, пустые строки — остаются нетронутыми

Такой подход гарантирует, что все комментарии и форматирование переживут переупорядочивание без изменений.
