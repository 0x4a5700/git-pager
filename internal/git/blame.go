package git

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var fortyHexRegex = regexp.MustCompile("^[0-9a-f]{40}$")

// BlameLine is one entry from `git blame --line-porcelain`: the
// commit, author, and time that last touched a given line of the
// file at a particular revision. Content is the literal line text
// with the porcelain leading tab already stripped.
type BlameLine struct {
	Hash      string
	ShortHash string
	Author    string
	Time      time.Time
	Content   string
}

// Blame returns per-line blame information for relPath at the given
// commit. The returned slice is parallel to the file's lines at that
// revision (one entry per line, in order).
func (s *Source) Blame(hash string) ([]BlameLine, error) {
	return BlameAt(s.repoDir, hash, s.relPath)
}

// BlameAt shells out to `git blame --line-porcelain` and parses the
// output into BlameLine entries. line-porcelain repeats the full
// header block for every line, trading a bit of output size for a
// dramatically simpler parser.
func BlameAt(repoDir, hash, relPath string) ([]BlameLine, error) {
	cmd := exec.Command("git", "-C", repoDir, "blame",
		"--line-porcelain", hash, "--", relPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame %s -- %s: %w", hash, relPath, err)
	}
	return parseBlamePorcelain(string(out))
}

// parseBlamePorcelain walks the output of `git blame --line-porcelain`
// and emits one BlameLine per content ('\t'-prefixed) line. Unknown
// header fields are ignored so future additions to the porcelain
// format do not break parsing.
func parseBlamePorcelain(s string) ([]BlameLine, error) {
	var lines []BlameLine
	scanner := bufio.NewScanner(strings.NewReader(s))
	// Content lines can be arbitrarily long; raise the scanner
	// buffer ceiling so we don't choke on generated files.
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)

	var cur BlameLine
	var unixTime int64
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "\t") {
			// Content row — finalise and flush.
			cur.Content = line[1:]
			if unixTime != 0 {
				cur.Time = time.Unix(unixTime, 0).UTC()
			}
			lines = append(lines, cur)
			cur = BlameLine{}
			unixTime = 0
			continue
		}
		// The first header line of a group is "<hash> <orig> <final>"
		// (optionally followed by a group size). We detect it by
		// cur.Hash being empty — the next flush resets that field.
		if cur.Hash == "" {
			if h, ok := parseBlameHashLine(line); ok {
				cur.Hash = h
				if len(h) >= 7 {
					cur.ShortHash = h[:7]
				}
				continue
			}
		}
		switch {
		case strings.HasPrefix(line, "author "):
			cur.Author = line[len("author "):]
		case strings.HasPrefix(line, "author-time "):
			if n, err := strconv.ParseInt(line[len("author-time "):], 10, 64); err == nil {
				unixTime = n
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("blame parse: %w", err)
	}
	return lines, nil
}

// parseBlameHashLine accepts the first line of a porcelain entry
// ("<40-hex> <orig-line> <final-line>[ <num-lines>]") and extracts
// the full hash. Returns ok=false for anything that doesn't look
// like one so the caller can safely fall through to field parsing.
func parseBlameHashLine(line string) (string, bool) {
	sp := strings.IndexByte(line, ' ')
	if sp != 40 {
		return "", false
	}
	hash := line[:40]

	if !fortyHexRegex.MatchString(hash) {
		return "", false
	}
	return hash, true
}
