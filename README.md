# Gas Town

**Enterprise-grade cognitive processing for AI agent swarms.**

Gas Town is a multi-agent workspace manager that coordinates AI coding agents working on software projects. It provides the infrastructure for spawning workers, processing work through structured workflows (molecules), and coordinating agents through a unified data plane (Beads).

## The Idea

Traditional AI coding assistants help you write code. Gas Town **writes code for you**.

Instead of AI as autocomplete, Gas Town treats AI agents as workers:
- **Polecats** implement features and fix bugs
- **Refineries** review and merge code
- **Witnesses** manage worker lifecycle
- **Mayors** coordinate across projects

Work flows through **molecules** - structured workflow templates that encode quality gates, dependencies, and recovery checkpoints. Any worker can continue any workflow from where another left off.

## Key Features

- **Nondeterministic Idempotence**: Workflows survive crashes, context compaction, and agent restarts
- **Molecule-Based Quality**: Structured workflows with built-in gates, not prompt-based instructions
- **Unified Data Plane**: All state in Beads (issues, messages, workflows) - queryable, auditable, persistent
- **Hierarchical Coordination**: Mayor → Witness → Refinery → Polecat chain of command
- **Federation Ready**: Multiple rigs across machines, coordinated through Beads sync

## Quick Start

```bash
# Install
go install github.com/steveyegge/gastown/cmd/gt@latest

# Create a town (workspace)
gt install ~/gt --git

# Add a rig for your project
gt rig add myproject --remote=https://github.com/you/myproject.git

# Spawn a polecat to work on an issue
gt spawn --issue myproject-123 --molecule mol-engineer-in-box
```

## Architecture

```
Town (~/gt/)
├── Mayor (global coordinator)
├── Rig: project-alpha
│   ├── Witness (lifecycle manager)
│   ├── Refinery (merge queue)
│   └── Polecats (workers)
│       ├── furiosa/
│       ├── nux/
│       └── slit/
└── Rig: project-beta
    └── ...
```

See [docs/architecture.md](docs/architecture.md) for comprehensive documentation.

## Molecules

Molecules are structured workflow templates:

```markdown
## Molecule: engineer-in-box
Full workflow from design to merge.

## Step: design
Think carefully about architecture.
Write a brief design summary.

## Step: implement
Write the code. Follow codebase conventions.
Needs: design

## Step: test
Write and run tests. Cover edge cases.
Needs: implement

## Step: submit
Submit for merge via refinery.
Needs: test
```

Built-in molecules:
- `mol-engineer-in-box` - Full quality workflow (design → implement → review → test → submit)
- `mol-quick-fix` - Fast path for small changes (implement → test → submit)
- `mol-research` - Exploration workflow (investigate → document)

## Beads

Gas Town uses [Beads](https://github.com/steveyegge/beads) for issue tracking and coordination:

```bash
bd ready              # Show work ready to start
bd list --status=open # All open issues
bd show gt-123        # Issue details
bd create --title="Fix auth bug" --type=bug
bd close gt-123       # Mark complete
```

## Commands

```bash
# Town management
gt install <path>     # Create new town
gt status             # Overall status
gt doctor             # Diagnose issues

# Rig management
gt rig add <name>     # Add project rig
gt rig list           # List rigs

# Worker management
gt spawn --issue <id> # Start polecat on issue
gt polecat list <rig> # List polecats

# Communication
gt mail inbox         # Check messages
gt mail send <addr>   # Send message
```

## Status

**Work in Progress** - This is the Go rewrite of the Python gastown tool.

See [gastown-py](https://github.com/steveyegge/gastown-py) for the Python version.

## Documentation

- [Architecture](docs/architecture.md) - System design and concepts
- [Vision](docs/vision.md) - Where Gas Town is going
- [Federation](docs/federation-design.md) - Multi-machine coordination
- [Merge Queue](docs/merge-queue-design.md) - Refinery and integration

## Development

```bash
# Build
go build -o gt ./cmd/gt

# Test
go test ./...

# Install locally
go install ./cmd/gt
```

## Related

- [beads](https://github.com/steveyegge/beads) - Issue tracking for AI agents
- [gastown-py](https://github.com/steveyegge/gastown-py) - Python version (reference)

## License

MIT
