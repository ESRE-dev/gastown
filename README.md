# Gas Town

**The IDE of 2026** - not Integrated Development Environment, but **Integrated Delegation Engine**.

Gas Town turns Claude Code (the Steam Engine) into a Steam Train, with Beads as the globally distributed railway network. Workers spawn, work molecules, submit to merge queues, and get cleaned up - all autonomously.

## The Vision

```
Claude       = Fire (the energy source)
Claude Code  = Steam Engine (harnesses the fire)
Gas Town     = Steam Train (coordinates engines on tracks)
Beads        = Railroad Tracks (the persistent ledger of work)
```

**Core principle: Gas Town is a Village.**

Not a rigid hierarchy with centralized monitoring, but an anti-fragile village where every agent understands the whole system and can help any neighbor. If you see something stuck, you can help. The village heals itself through distributed awareness.

## Install

```bash
go install github.com/steveyegge/gastown/cmd/gt@latest
```

## Quick Start

```bash
# Create a town (workspace)
gt install ~/gt

# Add a project rig
gt rig add myproject --remote=https://github.com/you/myproject.git

# Spawn a worker on an issue
gt spawn --issue myproject-123
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

- **Molecular Chemistry of Work**: Protos (templates) → Mols (flowing work) → Wisps (ephemeral) → Digests (outcomes)
- **Beads as Universal Data Plane**: Git-backed, human-readable, fractal ledger ([github.com/steveyegge/beads](https://github.com/steveyegge/beads))
- **Antifragility**: Self-monitoring village, not centralized hierarchy
- **Propulsion Principle**: Agents pull work from molecules, don't wait for commands
- **Nondeterministic Idempotence**: Any worker can continue any molecule after crashes

## Commands

```bash
gt status             # Town status
gt rig list           # List rigs
gt spawn --issue <id> # Start worker
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
