# Plan Package

The `plan` package provides markdown-to-epic conversion for GasTown's beads system. It parses markdown task lists and converts them into structured epic plans.

## Features

- **Markdown Parsing**: Parses headers, checkbox items, bullet lists, and numbered lists
- **Nested Structure**: Supports nested items and subsections
- **Sequential Dependencies**: Automatically creates phase dependencies (sections are sequential)
- **Parallel Tasks**: Checkbox items within a section are parallel by default
- **Numbered Lists**: Detected as sequential steps
- **Issue Type Detection**: Automatically determines issue types (task, bug, feature) from titles

## Usage

### Basic Example

```go
package main

import (
    "strings"
    "github.com/steveyegge/gastown/internal/plan"
)

func main() {
    markdown := `# Microservices Migration

## Phase 1: Setup
- [ ] Initialize Kubernetes cluster
- [ ] Configure monitoring

## Phase 2: Migration
1. Migrate auth service
2. Migrate user service
3. Migrate order service

## Phase 3: Testing
- [ ] Integration tests
- [ ] Load testing
`

    // Parse markdown
    doc, err := plan.ParseMarkdown(strings.NewReader(markdown))
    if err != nil {
        panic(err)
    }

    // Convert to epic
    epic, err := plan.Convert(doc)
    if err != nil {
        panic(err)
    }

    // epic now contains structured plan:
    // - Title: "Microservices Migration"
    // - 3 phases with sequential dependencies
    // - Phase 2 has sequential children (numbered list)
    // - Phase 1 and 3 have parallel children (checkboxes)
}
```

### Markdown Input Format

#### Headers as Phases
```markdown
# Epic Title

## Phase 1: Setup
## Phase 2: Implementation
## Phase 3: Testing
```

Headers become sections/phases. Sections at the same level are sequential (Phase 2 depends on Phase 1, etc.).

#### Checkbox Items (Parallel Tasks)
```markdown
## Phase 1
- [ ] Task A
- [ ] Task B
- [ ] Task C
```

Checkbox items are tasks that can be done in parallel.

#### Numbered Lists (Sequential Steps)
```markdown
## Phase 2
1. First step
2. Second step
3. Third step
```

Numbered lists indicate sequential steps.

#### Nested Items
```markdown
## Phase 1
- [ ] Parent task
  - [ ] Sub-task 1
  - [ ] Sub-task 2
    - [ ] Sub-sub-task
```

Indentation creates nested children (use 2-space indent).

#### Mixed Structure
```markdown
# Project Plan

## Phase 1: Setup
- [ ] Initialize database
  1. Create schema
  2. Seed data
  3. Configure backups
- [ ] Configure auth
  - [ ] OAuth setup
  - [ ] User management

## Phase 2: Implementation
1. Backend API
2. Frontend UI
3. Integration testing
```

## Data Structures

### PlanDocument
Represents the parsed markdown:
- `Title`: Document title (from H1)
- `Sections`: Top-level sections

### PlanSection
Represents a section/phase:
- `Title`: Section title
- `Level`: Header level (1-6)
- `Items`: Task items
- `Children`: Nested subsections

### PlanItem
Represents a task item:
- `Text`: Item text
- `Checked`: Whether checkbox is checked
- `IsCheckbox`: Whether this is a checkbox item
- `IsNumbered`: Whether from numbered list
- `Children`: Nested items

### EpicPlan
Structured epic output:
- `Title`: Epic title
- `Description`: Epic description
- `Children`: Child issues
- `Priority`: Epic priority (0-4)

### EpicChild
Represents a child issue:
- `Title`: Issue title
- `Description`: Issue description
- `Type`: "task", "bug", or "feature"
- `Priority`: 0-4
- `Children`: Nested children
- `DependsOn`: Indices of dependencies
- `Sequential`: Whether children are sequential

## Conversion Rules

1. **Phases are Sequential**: Top-level sections depend on previous sections
2. **Checkbox Items are Parallel**: Can be done in any order
3. **Numbered Items are Sequential**: Must be done in order
4. **Type Detection**: Based on keywords in title:
   - "bug", "fix" → `bug`
   - "feature", "add", "implement" → `feature`
   - "test", "qa" → `task`
   - "phase", "stage", "step" → `feature`
   - Default → `task`

## Testing

Run tests:
```bash
go test ./internal/plan/...
```

Run specific test:
```bash
go test ./internal/plan/ -run TestParseMarkdown_RealWorldExample -v
```
