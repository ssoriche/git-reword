package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ssoriche/git-reword/internal/reword"
)

const usage = `Usage: git-reword [flags] [<hash> <message>]...

Reword git commits without shell-quoting headaches.

Arguments:
  Pairs of <commit-hash> <new-message>

Flags:
  --from <file>   Read hash→message mapping as JSON (use "-" for stdin)
  --dry-run       Show what would happen without modifying the repo

Examples:
  git reword abc123 "New message"
  git reword abc123 "First" def456 "Second"
  git reword --from messages.json
  echo '{"abc123":"New msg"}' | git reword --from -
  git reword --dry-run abc123 "New message"
`

func main() {
	fromFlag := flag.String("from", "", "read hash→message mapping from JSON file (use \"-\" for stdin)")
	dryRun := flag.Bool("dry-run", false, "show what would happen without modifying the repo")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
	}
	flag.Parse()

	msgs, err := reword.LoadInput(flag.Args(), *fromFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := reword.Run(msgs, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
}
