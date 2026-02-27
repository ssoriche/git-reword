package reword

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testRepo creates a temporary git repo with n commits and returns the repo path
// and a slice of commit hashes (oldest first).
func testRepo(t *testing.T, n int) (string, []string) {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	run("git", "init")
	run("git", "config", "user.name", "Test")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "commit.gpgsign", "false")

	var hashes []string
	for i := range n {
		filename := string(rune('a'+i)) + ".txt"
		os.WriteFile(filepath.Join(dir, filename), []byte(filename), 0644)
		run("git", "add", filename)
		run("git", "commit", "-m", "commit "+filename)
		hash := run("git", "rev-parse", "HEAD")
		hashes = append(hashes, hash)
	}

	return dir, hashes
}

// allMessages returns commit messages from oldest to newest.
func allMessages(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "log", "--format=%B---", "--reverse")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("getting commit messages: %v\n%s", err, out)
	}
	raw := strings.TrimSpace(string(out))
	parts := strings.Split(raw, "---")
	var msgs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			msgs = append(msgs, p)
		}
	}
	return msgs
}

func setGitDir(t *testing.T, dir string) {
	t.Helper()
	old := gitDir
	gitDir = dir
	t.Cleanup(func() { gitDir = old })
}

func TestRewordSingleCommit(t *testing.T) {
	dir, hashes := testRepo(t, 3)
	setGitDir(t, dir)

	// Reword the second commit (index 1).
	msgs := map[string]string{
		hashes[1][:7]: "reworded second commit",
	}
	if err := Run(msgs, false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := allMessages(t, dir)
	if len(got) != 3 {
		t.Fatalf("expected 3 commits, got %d: %v", len(got), got)
	}
	if got[0] != "commit a.txt" {
		t.Errorf("commit 0: got %q, want %q", got[0], "commit a.txt")
	}
	if got[1] != "reworded second commit" {
		t.Errorf("commit 1: got %q, want %q", got[1], "reworded second commit")
	}
	if got[2] != "commit c.txt" {
		t.Errorf("commit 2: got %q, want %q", got[2], "commit c.txt")
	}
}

func TestRewordMultipleCommits(t *testing.T) {
	dir, hashes := testRepo(t, 4)
	setGitDir(t, dir)

	msgs := map[string]string{
		hashes[0][:7]: "first reworded",
		hashes[2][:7]: "third reworded",
	}
	if err := Run(msgs, false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := allMessages(t, dir)
	expected := []string{"first reworded", "commit b.txt", "third reworded", "commit d.txt"}
	if len(got) != len(expected) {
		t.Fatalf("expected %d commits, got %d: %v", len(expected), len(got), got)
	}
	for i, want := range expected {
		if got[i] != want {
			t.Errorf("commit %d: got %q, want %q", i, got[i], want)
		}
	}
}

func TestRewordSpecialCharacters(t *testing.T) {
	dir, hashes := testRepo(t, 3)
	setGitDir(t, dir)

	special := "feat: support \"quotes\", $variables, `backticks`, and 'single quotes'\n\nBody with special chars: <>&|;(){}[] and unicode: 日本語 🎉"
	msgs := map[string]string{
		hashes[1][:7]: special,
	}
	if err := Run(msgs, false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := allMessages(t, dir)
	if got[1] != special {
		t.Errorf("commit 1:\ngot:  %q\nwant: %q", got[1], special)
	}
}

func TestRewordDryRun(t *testing.T) {
	dir, hashes := testRepo(t, 3)
	setGitDir(t, dir)

	msgs := map[string]string{
		hashes[1][:7]: "should not appear",
	}
	if err := Run(msgs, true); err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}

	// Verify original message is unchanged.
	got := allMessages(t, dir)
	if got[1] != "commit b.txt" {
		t.Errorf("dry-run modified commit: got %q, want %q", got[1], "commit b.txt")
	}
}

func TestRewordDirtyTree(t *testing.T) {
	dir, _ := testRepo(t, 3)
	setGitDir(t, dir)

	// Dirty the working tree.
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("modified"), 0644)

	msgs := map[string]string{"abc1234": "msg"}
	err := Run(msgs, false)
	if err == nil {
		t.Fatal("expected error for dirty tree")
	}
	if !strings.Contains(err.Error(), "not clean") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRewordInvalidHash(t *testing.T) {
	dir, _ := testRepo(t, 3)
	setGitDir(t, dir)

	msgs := map[string]string{"deadbeef1234567": "msg"}
	err := Run(msgs, false)
	if err == nil {
		t.Fatal("expected error for invalid hash")
	}
	if !strings.Contains(err.Error(), "cannot resolve") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "single pair",
			args: []string{"abc123", "New message"},
			want: map[string]string{"abc123": "New message"},
		},
		{
			name: "two pairs",
			args: []string{"abc123", "First", "def456", "Second"},
			want: map[string]string{"abc123": "First", "def456": "Second"},
		},
		{
			name:    "odd number of args",
			args:    []string{"abc123"},
			wantErr: true,
		},
		{
			name:    "no args",
			args:    nil,
			wantErr: true,
		},
		{
			name:    "empty message",
			args:    []string{"abc123", ""},
			wantErr: true,
		},
		{
			name:    "duplicate hash",
			args:    []string{"abc123", "msg1", "abc123", "msg2"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseArgs() = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ParseArgs()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestParseJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "valid JSON",
			input: `{"abc123": "New message", "def456": "Another message"}`,
			want:  map[string]string{"abc123": "New message", "def456": "Another message"},
		},
		{
			name:    "empty object",
			input:   `{}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `{bad}`,
			wantErr: true,
		},
		{
			name:    "empty message value",
			input:   `{"abc123": ""}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseJSON(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseJSON() = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ParseJSON()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestLoadInputFromFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "messages.json")
	data := map[string]string{"abc123": "msg from file"}
	raw, _ := json.Marshal(data)
	os.WriteFile(f, raw, 0644)

	got, err := LoadInput(nil, f)
	if err != nil {
		t.Fatalf("LoadInput: %v", err)
	}
	if got["abc123"] != "msg from file" {
		t.Errorf("got %q, want %q", got["abc123"], "msg from file")
	}
}
