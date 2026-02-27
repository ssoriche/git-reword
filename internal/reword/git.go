package reword

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitDir is the working directory for git commands. Empty means current directory.
// Tests override this to point at temporary repos.
var gitDir string

func gitRun(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if gitDir != "" {
		cmd.Dir = gitDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// gitRunEnv runs a git command with additional environment variables.
func gitRunEnv(env []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if gitDir != "" {
		cmd.Dir = gitDir
	}
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func gitIsClean() error {
	out, err := gitRun("status", "--porcelain")
	if err != nil {
		return fmt.Errorf("checking working tree: %w", err)
	}
	if out != "" {
		return fmt.Errorf("working tree is not clean; commit or stash changes first")
	}
	return nil
}

func gitIsRebaseInProgress() bool {
	// Determine the .git directory location
	gitDirPath, err := gitRun("rev-parse", "--git-dir")
	if err != nil {
		return false
	}
	// If gitDir is set and gitDirPath is relative, make it absolute
	if gitDir != "" && !filepath.IsAbs(gitDirPath) {
		gitDirPath = filepath.Join(gitDir, gitDirPath)
	}
	for _, sub := range []string{"rebase-merge", "rebase-apply"} {
		if info, err := os.Stat(filepath.Join(gitDirPath, sub)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func gitResolveCommit(short string) (string, error) {
	hash, err := gitRun("rev-parse", "--verify", short+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("cannot resolve commit %q: %w", short, err)
	}
	return hash, nil
}

func gitRebaseHead() (string, error) {
	return gitRun("rev-parse", "REBASE_HEAD")
}
