package reword

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Run rewords the given commits. messages maps short (or full) hashes to their
// new commit messages. If dryRun is true, it prints what would happen without
// modifying the repository.
func Run(messages map[string]string, dryRun bool) error {
	// --- Preconditions ---
	if err := gitIsClean(); err != nil {
		return err
	}
	if gitIsRebaseInProgress() {
		return fmt.Errorf("a rebase is already in progress; finish or abort it first")
	}

	// --- Resolve hashes ---
	// Map resolved full hashes to messages, keeping a reverse lookup from full → short for logging.
	resolved := make(map[string]string, len(messages))
	shortOf := make(map[string]string, len(messages))
	for short, msg := range messages {
		full, err := gitResolveCommit(short)
		if err != nil {
			return err
		}
		if _, dup := resolved[full]; dup {
			return fmt.Errorf("duplicate commit: %s and another input both resolve to %s", short, full[:12])
		}
		resolved[full] = msg
		shortOf[full] = short
	}

	// --- Determine commit order and rebase base ---
	// Get all commits reachable from HEAD as an ordered list (newest first).
	revListOut, err := gitRun("rev-list", "HEAD")
	if err != nil {
		return fmt.Errorf("listing commits: %w", err)
	}
	allCommits := strings.Split(revListOut, "\n")

	// Filter to just our target commits, preserving chronological order (oldest first).
	var ordered []string
	for i := len(allCommits) - 1; i >= 0; i-- {
		if _, ok := resolved[allCommits[i]]; ok {
			ordered = append(ordered, allCommits[i])
		}
	}
	if len(ordered) != len(resolved) {
		// Some commits weren't found in HEAD's history.
		var missing []string
		found := make(map[string]bool, len(ordered))
		for _, h := range ordered {
			found[h] = true
		}
		for full := range resolved {
			if !found[full] {
				missing = append(missing, shortOf[full])
			}
		}
		return fmt.Errorf("commits not in current branch history: %s", strings.Join(missing, ", "))
	}

	// The base is the parent of the earliest (oldest) target commit.
	earliest := ordered[0]
	base, err := gitRun("rev-parse", earliest+"^")
	if err != nil {
		// If there's no parent, use --root
		base = "--root"
	}

	// --- Dry run ---
	if dryRun {
		fmt.Println("Dry run — the following commits would be reworded:")
		for _, full := range ordered {
			short := shortOf[full]
			// Show first line of new message.
			firstLine := strings.SplitN(resolved[full], "\n", 2)[0]
			fmt.Printf("  %s → %s\n", short, firstLine)
		}
		fmt.Printf("Rebase base: %s\n", base)
		return nil
	}

	// --- Build sequence editor ---
	// The sequence editor script changes "pick <hash>" to "edit <hash>" for our targets.
	// We match on hash prefixes since the todo list uses abbreviated hashes.
	seqEditor := buildSequenceEditor(ordered)

	slog.Debug("starting rebase", "base", base, "targets", len(ordered))

	// --- Start interactive rebase ---
	var rebaseArgs []string
	if base == "--root" {
		rebaseArgs = []string{"rebase", "-i", "--root"}
	} else {
		rebaseArgs = []string{"rebase", "-i", base}
	}
	_, err = gitRunEnv(
		[]string{"GIT_SEQUENCE_EDITOR=" + seqEditor},
		rebaseArgs...,
	)
	if err != nil {
		// The rebase will stop at the first "edit" commit — that's expected, not an error.
		// Check if we're now in a rebase.
		if !gitIsRebaseInProgress() {
			return fmt.Errorf("rebase failed to start: %w", err)
		}
	}

	// --- Rebase loop: amend and continue ---
	for range len(ordered) + 10 { // +10 as safety margin
		if !gitIsRebaseInProgress() {
			// Rebase finished.
			break
		}

		head, err := gitRebaseHead()
		if err != nil {
			// REBASE_HEAD not available — might be a conflict or done.
			abortErr := abort()
			return fmt.Errorf("cannot determine REBASE_HEAD: %w (abort: %v)", err, abortErr)
		}

		msg, ok := resolved[head]
		if !ok {
			// We stopped at a commit we didn't ask to edit — shouldn't happen.
			abortErr := abort()
			return fmt.Errorf("stopped at unexpected commit %s (abort: %v)", head[:12], abortErr)
		}

		slog.Debug("amending commit", "hash", head[:12], "short", shortOf[head])

		// Write message to a temp file to avoid any shell quoting issues.
		if err := amendWithMessage(msg); err != nil {
			abortErr := abort()
			return fmt.Errorf("amending commit %s: %w (abort: %v)", shortOf[head], err, abortErr)
		}

		// Continue the rebase.
		_, err = gitRunEnv(
			[]string{"GIT_EDITOR=true"},
			"rebase", "--continue",
		)
		if err != nil {
			// Error is expected when there are more "edit" stops.
			if !gitIsRebaseInProgress() {
				// Rebase finished despite the error code (git sometimes exits non-zero
				// when the last continue completes).
				break
			}
			// Still in rebase — continue the loop for the next stop.
		}
	}

	if gitIsRebaseInProgress() {
		abortErr := abort()
		return fmt.Errorf("rebase did not complete after processing all commits (abort: %v)", abortErr)
	}

	fmt.Printf("Successfully reworded %d commit(s)\n", len(ordered))
	return nil
}

// buildSequenceEditor returns a shell command suitable for GIT_SEQUENCE_EDITOR
// that replaces "pick" with "edit" for the given full commit hashes.
func buildSequenceEditor(hashes []string) string {
	// Build a sed command that matches the first 7+ chars of each target hash.
	// The todo list uses abbreviated hashes so we match on prefixes.
	var parts []string
	for _, h := range hashes {
		prefix := h[:7]
		// sed: replace "pick <prefix>" with "edit <prefix>" (only the first word).
		parts = append(parts, fmt.Sprintf("s/^pick %s/edit %s/", prefix, prefix))
	}
	return "sed -i.bak " + shellQuote(strings.Join(parts, ";"))
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// amendWithMessage writes msg to a temp file and runs git commit --amend -F <file>.
func amendWithMessage(msg string) error {
	tmp, err := os.CreateTemp("", "git-reword-msg-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(msg); err != nil {
		tmp.Close()
		return fmt.Errorf("writing message: %w", err)
	}
	tmp.Close()

	_, err = gitRun("commit", "--amend", "--no-verify", "-F", tmp.Name())
	return err
}

func abort() error {
	_, err := gitRun("rebase", "--abort")
	return err
}
