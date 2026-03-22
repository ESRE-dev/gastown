// Gas Town OpenCode plugin: hooks SessionStart/Compaction via events,
// injects gt prime context into system prompt, drains inter-agent mail
// at turn boundaries, and propagates GasTown env vars to bash tool calls.
export const GasTown = async ({ $, directory }) => {
  const role = (process.env.GT_ROLE || "").toLowerCase();
  const autonomousRoles = new Set(["polecat", "witness", "refinery", "deacon"]);
  let didInit = false;

  // Promise-based context loading ensures the system transform hook can
  // await the result even if session.created hasn't resolved yet.
  let primePromise = null;

  const captureRun = async (cmd) => {
    try {
      // .text() captures stdout as a string and suppresses terminal echo.
      return await $`/bin/sh -lc ${cmd}`.cwd(directory).text();
    } catch (err) {
      console.error(`[gastown] ${cmd} failed`, err?.message || err);
      return "";
    }
  };

  const loadPrime = async () => {
    let context = await captureRun("gt prime --hook");
    if (autonomousRoles.has(role)) {
      const mail = await captureRun("gt mail check --inject");
      if (mail) {
        context += "\n" + mail;
      }
    }
    // NOTE: session-started nudge to deacon removed — it interrupted
    // the deacon's await-signal backoff. Deacon wakes on beads activity.
    return context;
  };

  return {
    event: async ({ event }) => {
      if (event?.type === "session.created") {
        if (didInit) return;
        didInit = true;
        // Start loading prime context early; system.transform will await it.
        primePromise = loadPrime();
      }
      if (event?.type === "session.compacted") {
        // Reset so next system.transform gets fresh context.
        primePromise = loadPrime();
      }
      if (event?.type === "session.deleted") {
        const sessionID = event.properties?.info?.id;
        if (sessionID) {
          await captureRun(`gt costs record --session ${sessionID}`);
        }
      }
    },
    "experimental.chat.system.transform": async (input, output) => {
      // If session.created hasn't fired yet, start loading now.
      if (!primePromise) {
        primePromise = loadPrime();
      }
      const context = await primePromise;
      if (context) {
        output.system.push(context);
      } else {
        // Reset so next transform retries instead of pushing empty forever.
        primePromise = null;
      }
    },
    "experimental.session.compacting": async ({ sessionID }, output) => {
      const roleDisplay = role || "unknown";
      output.context.push(`
## Gas Town Multi-Agent System

**After Compaction:** Run \`gt prime\` to restore full context.
**Check Hook:** \`gt hook\` - if work present, execute immediately (GUPP).
**Role:** ${roleDisplay}
`);
    },
    // Drain inter-agent mail on every user message (turn boundary).
    // Equivalent of Claude Code's UserPromptSubmit hook. Runs for ALL roles,
    // not just autonomous — any agent can receive nudges.
    "chat.message": async (input, output) => {
      try {
        const mail = await captureRun("gt mail check --inject");
        if (mail && mail.trim()) {
          output.parts.push({
            type: "text",
            text: mail.trim(),
            id: `gt-mail-${Date.now()}`,
            sessionID: input.sessionID,
            messageID: input.messageID || `gt-msg-${Date.now()}`,
          });
        }
      } catch {
        // Silently fail — mail delivery is best-effort.
      }
    },
    // Pre-tool-execution guard for bash commands. Mirrors Claude Code's PreToolUse
    // matchers from DefaultBase() in internal/hooks/config.go. Blocks dangerous
    // commands by replacing them with an informative error echo.
    "tool.execute.before": async (input, output) => {
      // Only guard bash tool calls.
      if (input.tool !== "bash" || !output.args?.command) return;

      const cmd = output.args.command;

      // pr-workflow guards (Claude: Bash(gh pr create*), Bash(git checkout -b*), etc.)
      const prWorkflow = ["gh pr create", "git checkout -b", "git switch -c"];
      // dangerous-command guards (Claude: Bash(rm -rf /*), Bash(git push --force*), etc.)
      // Note: "git push --force-with-lease" is the safer variant and should NOT be blocked.
      const dangerous = ["rm -rf /", "git push -f"];
      const isDangerous =
        dangerous.some((p) => cmd.includes(p)) ||
        (cmd.includes("git push --force") &&
          !cmd.includes("--force-with-lease"));

      let guardType = null;
      if (prWorkflow.some((p) => cmd.includes(p))) {
        guardType = "pr-workflow";
      } else if (isDangerous) {
        guardType = "dangerous-command";
      }

      if (!guardType) return;

      try {
        const proc = await $`/bin/sh -lc ${`gt tap guard ${guardType}`}`
          .cwd(directory)
          .nothrow();
        if (proc.exitCode !== 0) {
          const reason =
            proc.stderr?.toString().trim() ||
            proc.stdout?.toString().trim() ||
            `blocked by ${guardType} guard`;
          // Mutate the command property in-place (full reassignment of
          // output.args does NOT work — only property mutation propagates).
          output.args.command = `printf '%s\n' 'BLOCKED by GasTown guard (${guardType}): ${reason.replace(/'/g, "'\\''")}'; exit 1`;
        }
      } catch (err) {
        console.error("[gastown] gt tap guard failed:", err?.message || err);
        // Fail-open: if the guard check itself errors, allow the command.
      }
    },
    // Propagate GasTown environment variables to bash tool calls and PTY
    // sessions so gt/bd subcommands have proper context.
    //
    // Prefixes:
    //   GT_*    — core GasTown config (GT_ROLE, GT_RIG, GT_ROOT, GT_DOLT_PORT, …)
    //   BD_*    — beads identity & config (BD_ACTOR, BD_DOLT_AUTO_COMMIT, BD_OTEL_*, …)
    //   BEADS_* — beads runtime (BEADS_DIR, BEADS_AGENT_NAME, BEADS_DOLT_PORT, …)
    //   OTEL_*  — OpenTelemetry (OTEL_RESOURCE_ATTRIBUTES, OTEL_METRICS_EXPORTER, …)
    //
    // Discrete vars (set by AgentEnv but not prefix-matched above):
    //   GIT_AUTHOR_NAME, GIT_CEILING_DIRECTORIES — git attribution & safety
    "shell.env": async (input, output) => {
      const prefixes = ["GT_", "BD_", "BEADS_", "OTEL_"];
      const discrete = new Set(["GIT_AUTHOR_NAME", "GIT_CEILING_DIRECTORIES"]);
      for (const [key, value] of Object.entries(process.env)) {
        if (prefixes.some((p) => key.startsWith(p)) || discrete.has(key)) {
          output.env[key] = value;
        }
      }
    },
  };
};
