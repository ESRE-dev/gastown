# Gas Town

> **Status**: Experimental (v0.1) - We're exploring these ideas and invite you to explore with us.

Gas Town is an experiment in multi-agent coordination for Claude Code. It provides infrastructure for spawning workers, tracking work via molecules, and coordinating merges.

We think of it using steam-age metaphors:

```
Claude       = Fire (the energy source)
Claude Code  = Steam Engine (harnesses the fire)
Gas Town     = Steam Train (coordinates engines on tracks)
Beads        = Railroad Tracks (the persistent ledger)
```

The goal is a "village" architecture - not rigid hierarchy, but distributed awareness where agents can help neighbors when something is stuck. Whether this actually works at scale is something we're still discovering.

## Prerequisites

- **Go 1.23+** - For building from source
- **Git** - For rig management and beads sync
- **tmux** - Required for agent sessions (all workers run in tmux panes)
- **Claude Code CLI** - Required for agents (`claude` command must be available)

## Install

**From source (recommended for now):**

```bash
go install github.com/steveyegge/gastown/cmd/gt@latest
```

**Package managers (coming soon):**

```bash
# Homebrew (macOS/Linux)
brew install gastown

# npm (cross-platform)
npm install -g @anthropic/gastown
```

## Quick Start

```bash
# Create a town (workspace)
gt install ~/gt

# Add a project rig
gt rig add myproject --remote=https://github.com/you/myproject.git

# Assign work to a polecat
gt sling myproject-123 myproject
```

## Architecture

```
Town (~/gt/)
├── Mayor (global coordinator)
└── Rig: myproject
    ├── Witness (lifecycle manager)
    ├── Refinery (merge queue)
    └── Polecats (workers)
```

## Key Concepts

Ideas we're exploring:

- **Molecular Chemistry of Work**: Protos (templates) → Mols (flowing work) → Wisps (ephemeral) → Digests (outcomes)
- **Beads**: Git-backed, human-readable ledger for tracking work ([github.com/steveyegge/beads](https://github.com/steveyegge/beads))
- **Village Model**: Distributed awareness instead of centralized monitoring
- **Propulsion Principle**: Agents pull work from molecules rather than waiting for commands
- **Nondeterministic Idempotence**: The idea that any worker can continue any molecule after crashes

Some of these are implemented; others are still aspirational. See docs for current status.

## Commands

```bash
gt status             # Town status
gt rig list           # List rigs
gt sling <bead> <rig> # Assign work to polecat
gt mail inbox         # Check messages
gt peek <worker>      # Check worker health
gt nudge <worker>     # Wake stuck worker
```

## Documentation

- [Vision](docs/vision.md) - Core innovations and philosophy
- [Architecture](docs/architecture.md) - System design
- [Molecular Chemistry](docs/molecular-chemistry.md) - Work composition
- [Molecules](docs/molecules.md) - Workflow templates

## Development

```bash
go build -o gt ./cmd/gt
go test ./...
```

## License

MIT
