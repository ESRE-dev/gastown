# Crew tmux Configuration

> **Status**: Personal Workflow
> **Location**: Your personal dotfiles (e.g., `~/.emacs.d/tmux.conf`)

## Overview

Crew workers are persistent identities. Unlike polecats (ephemeral, witness-managed),
crew members keep their names and workspaces across sessions. This makes them ideal
candidates for hardwired tmux keybindings.

This document explains how to configure your personal tmux config to enable quick
cycling between crew sessions within a rig.

## Why Personal Config?

Crew session linking is **personal workflow**, not core Gas Town infrastructure:

- Your crew members are stable identities you control
- The groupings reflect how *you* want to work
- Different users may have different crew setups
- Keeps Gas Town codebase focused on agent mechanics

Store this in your personal dotfiles repo (e.g., `~/.emacs.d/`) for:
- Version control
- Sharing across machines
- Separation from project code

## Setup

### 1. Create tmux.conf in your dotfiles

```bash
# Example: ~/.emacs.d/tmux.conf
```

### 2. Symlink from home

```bash
ln -s ~/.emacs.d/tmux.conf ~/.tmux.conf
```

### 3. Configure crew cycling groups

The key insight: use `run-shell` with a case statement to route `C-b n`/`C-b p`
based on the current session name.

```tmux
# Crew session cycling - hardwired groups
# Group 1: gastown crew (max <-> joe)
# Group 2: beads crew (dave -> emma -> zoey -> dave)

bind n run-shell ' \
  s="#{session_name}"; \
  case "$s" in \
    gt-gastown-crew-max) tmux switch-client -t gt-gastown-crew-joe ;; \
    gt-gastown-crew-joe) tmux switch-client -t gt-gastown-crew-max ;; \
    gt-beads-crew-dave) tmux switch-client -t gt-beads-crew-emma ;; \
    gt-beads-crew-emma) tmux switch-client -t gt-beads-crew-zoey ;; \
    gt-beads-crew-zoey) tmux switch-client -t gt-beads-crew-dave ;; \
    *) tmux switch-client -n ;; \
  esac'

bind p run-shell ' \
  s="#{session_name}"; \
  case "$s" in \
    gt-gastown-crew-max) tmux switch-client -t gt-gastown-crew-joe ;; \
    gt-gastown-crew-joe) tmux switch-client -t gt-gastown-crew-max ;; \
    gt-beads-crew-dave) tmux switch-client -t gt-beads-crew-zoey ;; \
    gt-beads-crew-emma) tmux switch-client -t gt-beads-crew-dave ;; \
    gt-beads-crew-zoey) tmux switch-client -t gt-beads-crew-emma ;; \
    *) tmux switch-client -p ;; \
  esac'
```

### 4. Reload config

```bash
tmux source-file ~/.tmux.conf
```

## Session Naming Convention

Gas Town uses predictable session names:

```
gt-<rig>-crew-<name>
```

Examples:
- `gt-gastown-crew-max`
- `gt-gastown-crew-joe`
- `gt-beads-crew-dave`
- `gt-beads-crew-emma`
- `gt-beads-crew-zoey`

This predictability enables hardwired keybindings.

## Adding New Crew Members

When you add a new crew member:

1. Add entries to both `bind n` and `bind p` case statements
2. Maintain the cycle order (n goes forward, p goes backward)
3. Reload config: `tmux source-file ~/.tmux.conf`

Example - adding `frank` to gastown crew:

```tmux
# In bind n:
gt-gastown-crew-max) tmux switch-client -t gt-gastown-crew-joe ;;
gt-gastown-crew-joe) tmux switch-client -t gt-gastown-crew-frank ;;
gt-gastown-crew-frank) tmux switch-client -t gt-gastown-crew-max ;;

# In bind p (reverse order):
gt-gastown-crew-max) tmux switch-client -t gt-gastown-crew-frank ;;
gt-gastown-crew-joe) tmux switch-client -t gt-gastown-crew-max ;;
gt-gastown-crew-frank) tmux switch-client -t gt-gastown-crew-joe ;;
```

## Fallback Behavior

The `*) tmux switch-client -n ;;` fallback means:
- In a crew session → cycles within your group
- In any other session (mayor, witness, refinery) → standard all-session cycling

This keeps the default behavior for non-crew contexts.

## Starting Crew Sessions

When starting crew sessions manually (not through Gas Town spawn), remember to
configure the status line:

```bash
# Start session
tmux new-session -d -s gt-<rig>-crew-<name> -c /path/to/crew/<name>

# Configure status (Gas Town normally does this automatically)
tmux set-option -t gt-<rig>-crew-<name> status-left-length 25
tmux set-option -t gt-<rig>-crew-<name> status-left "👷 <rig>/crew/<name> "

# Start Claude
tmux send-keys -t gt-<rig>-crew-<name> 'claude' Enter
```

## Tips

- **Two-member groups**: For pairs like max/joe, n and p do the same thing (toggle)
- **Larger groups**: n cycles forward, p cycles backward
- **Mixed rigs**: Each rig's crew is a separate group - no cross-rig cycling
- **Testing**: Use `tmux display-message -p '#{session_name}'` to verify session names

## Related

- [session-lifecycle.md](session-lifecycle.md) - How sessions cycle
- [propulsion-principle.md](propulsion-principle.md) - The "RUN IT" protocol
