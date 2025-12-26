# Formula Evolution: Predictions and Future Directions

> Written: 2024-12-26 (Day 5 of formulas, Day 10 of molecules)
> Status: Speculative - institutional knowledge capture

## Context

Gas Town's workflow system has evolved rapidly:
- **Day 0**: Molecules conceived as workflow instances
- **Day 5**: Formulas introduced as compile-time templates
- **Day 10**: Patrols, session persistence, bond operators, protomolecules

We've essentially discovered molecular chemistry for workflows. This document captures predictions about where this goes next.

## Current Architecture

```
Formulas (TOML)     →  compile  →  Molecules (runtime)
   ↓                                    ↓
Templates with                     Executable workflows
{{variables}}                      with state persistence
   ↓                                    ↓
Stored in                          Walked by patrols,
.beads/formulas/                   survive restarts
```

## The Package Ecosystem Parallel

Every successful package ecosystem evolved through similar phases:

| Phase | npm | Maven | Go | Formulas (predicted) |
|-------|-----|-------|-----|---------------------|
| 1. Local files | `./lib/` | `lib/*.jar` | `vendor/` | `.beads/formulas/` |
| 2. Central registry | npmjs.org | Maven Central | proxy.golang.org | Mol Mall |
| 3. Namespacing | `@org/pkg` | `groupId:artifactId` | `github.com/org/pkg` | `@org/formula` |
| 4. Version locking | `package-lock.json` | `<version>` in pom | `go.sum` | `formulas.lock`? |
| 5. Composition | peer deps, plugins | BOM, parent POMs | embedding | nesting, AOP |

## Formula Resolution Hierarchy

Like `$PATH` or module resolution, formulas should resolve through layers:

```
1. Project:  ./.beads/formulas/          # Project-specific workflows
2. Rig:      ~/gt/<rig>/.formulas/       # Rig-wide (shared across clones)
3. Town:     ~/gt/.formulas/             # Town-wide (mayor's standard library)
4. User:     ~/.gastown/formulas/        # Personal collection
5. Mall:     mol-mall.io/                # Published packages
```

**Resolution order**: project → rig → town → user → mall (first match wins)

**Explicit scoping**: `@gastown/shiny` always resolves to Mol Mall's gastown org

## Formula Combinators

Current composition is implicit (nesting, `needs` dependencies). We should formalize **formula combinators** - higher-order operations on workflows:

### Sequential Composition
```
A >> B                    # A then B
release >> announce       # Release, then announce
```

### Parallel Composition
```
A | B                     # A and B concurrent (join before next)
(build-linux | build-mac | build-windows) >> package
```

### Conditional Composition
```
A ? B : C                 # if A succeeds then B else C
test ? deploy : rollback
```

### Wrapping (AOP)
```
wrap(A, with=B)           # B's before/after around A's steps
wrap(deploy, with=compliance-audit)
```

### Injection
```
inject(A, at="step-id", B)    # Insert B into A at specified step
inject(polecat-work, at="issue-work", shiny)
```

The Shiny-wrapping-polecat example: a polecat runs `polecat-work` formula, but the "issue-work" step (where actual coding happens) gets replaced/wrapped with `shiny`, adding design-review-test phases.

### Extension/Override
```
B extends A {
  override step "deploy" { ... }
  add step "notify" after "deploy" { ... }
}
```

### Algebra Properties

These combinators should satisfy algebraic laws:
- **Associativity**: `(A >> B) >> C = A >> (B >> C)`
- **Identity**: `A >> noop = noop >> A = A`
- **Parallel commutativity**: `A | B = B | A` (order doesn't matter)

## Higher Abstractions (Speculative)

### 1. Formula Algebras
Formal composition with provable guarantees:
```toml
[guarantees]
compliance = "all MRs pass audit step"
coverage = "test step runs before any deploy"
```

The system could verify these properties statically.

### 2. Constraint Workflows
Declare goals, let the system derive steps:
```toml
[goals]
reviewed = true
tested = true
compliant = true
deployed_to = "production"

# System generates: review >> test >> compliance >> deploy
```

### 3. Adaptive Formulas
Learn from execution history:
```toml
[adapt]
on_failure_rate = { threshold = 0.3, action = "add-retry" }
on_slow_step = { threshold = "5m", action = "parallelize" }
```

### 4. Formula Schemas/Interfaces
Type system for workflows:
```toml
implements = ["Reviewable", "Deployable"]

# Reviewable requires: has step with tag "review"
# Deployable requires: has step with tag "deploy", depends on "test"
```

Enables: "Give me all formulas that implement Deployable"

### 5. Meta-Formulas
Formulas that generate formulas:
```toml
[meta]
for_each = ["service-a", "service-b", "service-c"]
generate = "deploy-{{item}}"
template = "microservice-deploy"
```

## Mol Mall Architecture (Predicted)

### Registry Structure
```
mol-mall.io/
├── @gastown/           # Official Gas Town formulas
│   ├── shiny
│   ├── polecat-work
│   └── release
├── @acme-corp/         # Enterprise org (private)
│   └── compliance-review
├── @linus/             # Individual publisher
│   └── kernel-review
└── community/          # Unscoped public formulas
    └── towers-of-hanoi
```

### Package Manifest
```toml
# formula.toml or mol.toml
[package]
name = "shiny"
version = "1.2.0"
description = "Engineer in a Box"
authors = ["Gas Town <gastown@example.com>"]
license = "MIT"

[dependencies]
"@gastown/review-patterns" = "^2.0"

[peer-dependencies]
# Must be provided by the consuming workflow
"@gastown/test-runner" = "*"
```

### Lock File
```toml
# formulas.lock
[[package]]
name = "@gastown/shiny"
version = "1.2.0"
checksum = "sha256:abc123..."
source = "mol-mall.io"

[[package]]
name = "@gastown/review-patterns"
version = "2.1.3"
checksum = "sha256:def456..."
source = "mol-mall.io"
```

## Distribution Scenarios

### Scenario 1: Celebrity Formulas
"Linus Torvalds' kernel review formula" - expert-curated workflows that encode deep domain knowledge. High signal, community trust.

### Scenario 2: Enterprise Compliance
Internal company formula for compliance reviews. Private registry, mandatory inclusion in all MR workflows via policy.

### Scenario 3: Standard Library
Gas Town ships `@gastown/*` formulas as blessed defaults. Like Go's standard library - high quality, well-maintained, always available.

### Scenario 4: Community Ecosystem
Thousands of community formulas of varying quality. Need: ratings, downloads, verified publishers, security scanning.

## Open Questions

1. **Versioning semantics**: SemVer? How do breaking changes work for workflows?

2. **Security model**: Formulas execute code. Sandboxing? Permissions? Signing?

3. **Testing formulas**: How do you unit test a workflow? Mock steps?

4. **Formula debugging**: Step through execution? Breakpoints?

5. **Rollback semantics**: If step 7 of 10 fails, what's the undo story?

6. **Cross-rig formulas**: Can a gastown formula reference a beads formula?

7. **Formula inheritance depth**: How deep can `extends` chains go? Diamond problem?

## Next Steps (Suggested)

1. **Formalize combinator semantics** - Write spec for `>>`, `|`, `wrap`, `inject`
2. **Prototype Mol Mall** - Even a simple file-based registry to prove the model
3. **Add formula schemas** - `implements` field with interface definitions
4. **Build formula test harness** - Run formulas in dry-run/mock mode

---

*This document captures thinking as of late December 2024. Formulas are evolving rapidly - update as we learn more.*
