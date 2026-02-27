package reword

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ParseArgs parses positional arguments as alternating hash/message pairs.
func ParseArgs(args []string) (map[string]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no arguments provided")
	}
	if len(args)%2 != 0 {
		return nil, fmt.Errorf("arguments must be pairs of <hash> <message>; got %d args", len(args))
	}
	msgs := make(map[string]string, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		hash := args[i]
		msg := args[i+1]
		if hash == "" {
			return nil, fmt.Errorf("empty hash at argument %d", i+1)
		}
		if msg == "" {
			return nil, fmt.Errorf("empty message for hash %q", hash)
		}
		if _, dup := msgs[hash]; dup {
			return nil, fmt.Errorf("duplicate hash %q", hash)
		}
		msgs[hash] = msg
	}
	return msgs, nil
}

// ParseJSON reads a JSON object mapping short hashes to messages from r.
func ParseJSON(r io.Reader) (map[string]string, error) {
	var msgs map[string]string
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&msgs); err != nil {
		return nil, fmt.Errorf("parsing JSON input: %w", err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("JSON input contains no commit mappings")
	}
	for hash, msg := range msgs {
		if hash == "" {
			return nil, fmt.Errorf("empty hash in JSON input")
		}
		if msg == "" {
			return nil, fmt.Errorf("empty message for hash %q in JSON input", hash)
		}
	}
	return msgs, nil
}

// LoadInput dispatches between CLI args and JSON file/stdin based on fromFlag.
// If fromFlag is set, it reads JSON from the named file (or stdin if "-").
// Otherwise, it parses args as positional hash/message pairs.
func LoadInput(args []string, fromFlag string) (map[string]string, error) {
	if fromFlag != "" {
		var r io.Reader
		if fromFlag == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(fromFlag)
			if err != nil {
				return nil, fmt.Errorf("opening input file: %w", err)
			}
			defer f.Close()
			r = f
		}
		return ParseJSON(r)
	}
	return ParseArgs(args)
}
