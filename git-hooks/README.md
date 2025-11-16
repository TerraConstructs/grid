# Beads git hooks

This directory contains git hooks that integrate bd (beads) with your git workflow, solving the race condition between daemon auto-flush and git commits.

## The Problem

When using bd in daemon mode, operations trigger a 5-second debounced auto-flush to JSONL. This creates a race condition:

1. User closes issue via MCP → daemon schedules flush (5 sec delay)
2. User commits code changes → JSONL appears clean
3. Daemon flush fires → JSONL modified after commit
4. Result: dirty working tree showing JSONL changes


Ref: [steveyegge/beads - examples/git-hooks](https://github.com/steveyegge/beads/tree/main/examples/git-hooks)
