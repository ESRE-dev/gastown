---
description: Hand off to fresh session, work continues from hook
allowed-tools: Bash(gt handoff:*)
argument-hint: [message]
---

Hand off to a fresh session.

User's handoff message (if any): $ARGUMENTS

Execute the appropriate command:
- If user provided a message: `gt handoff --cycle -m "USER_MESSAGE_HERE"`
- If no message provided: `gt handoff --cycle`

End watch. A new session takes over, picking up any molecule on the hook.
