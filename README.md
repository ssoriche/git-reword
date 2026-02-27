# git-reword

Reword git commit messages without shell-quoting headaches.

`git-reword` performs a non-interactive `git rebase -i` under the hood, automatically changing `pick` to `edit` for the target commits, amending each with the new message, and continuing the rebase. Messages are passed via temp files, so special characters like quotes, backticks, `$variables`, and unicode all work reliably.

## Why

Rewriting commit messages during an interactive rebase is tedious and error-prone, especially when messages contain shell metacharacters. `git-reword` makes it a single command — ideal for scripting, CI, and AI-assisted workflows.

## Installation

### Go install

```sh
go install github.com/ssoriche/git-reword/cmd/git-reword@latest
```

### Build from source

```sh
git clone https://github.com/ssoriche/git-reword.git
cd git-reword
devbox shell   # or use system Go 1.24+
make install
```

## Usage

Reword a single commit:

```sh
git reword abc123 "feat: new commit message"
```

Reword multiple commits at once:

```sh
git reword abc123 "first message" def456 "second message"
```

Read commit mappings from a JSON file:

```sh
git reword --from messages.json
```

Read from stdin:

```sh
echo '{"abc123":"fix: corrected typo"}' | git reword --from -
```

Preview what would happen without modifying the repo:

```sh
git reword --dry-run abc123 "new message"
```

## JSON format

The `--from` flag accepts a JSON object mapping commit hashes (short or full) to new messages:

```json
{
    "abc1234": "feat: add user authentication",
    "def5678": "fix: handle nil pointer in parser\n\nThe parser now checks for nil before dereferencing."
}
```

## How it works

1. Resolves all input hashes to full SHA-1s
2. Finds the earliest target commit and determines the rebase base (its parent)
3. Starts `git rebase -i` with a custom `GIT_SEQUENCE_EDITOR` that swaps `pick` → `edit` for target commits
4. At each `edit` stop, writes the new message to a temp file and runs `git commit --amend -F <file>`
5. Continues the rebase until all commits are reworded

If anything goes wrong mid-rebase, `git-reword` aborts the rebase and reports the error.

## Claude Code skill

You can use `git-reword` as a Claude Code slash command for AI-assisted commit rewriting. Save the following as `~/.claude/commands/reword-commits.md` (global) or `.claude/commands/reword-commits.md` (per-project):

````markdown
---
allowed-tools: Bash(git-reword:*), Bash(git log:*), Bash(git reword:*), Bash(go run:*)
---

Help me reword commits on my current branch.

1. Run `git log --oneline -20` to show recent commits
2. Ask me which commits to reword and what the new messages should be
3. Build a JSON object mapping each commit hash to its new message
4. Run `git-reword --from -` with the JSON piped to stdin

If `git-reword` is not installed, fall back to:
```
go run github.com/ssoriche/git-reword/cmd/git-reword@latest --from -
```
````

Then invoke it in Claude Code with `/reword-commits`.

## License

MIT
