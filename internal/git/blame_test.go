package git

import (
	"strings"
	"testing"
	"time"
)

func TestParseBlamePorcelain_SingleLine(t *testing.T) {
	// Minimal synthetic porcelain output for one content line.
	in := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef 1 1 1\n" +
		"author Alice Example\n" +
		"author-mail <alice@example.com>\n" +
		"author-time 1700000000\n" +
		"author-tz +0000\n" +
		"committer Alice Example\n" +
		"committer-mail <alice@example.com>\n" +
		"committer-time 1700000000\n" +
		"committer-tz +0000\n" +
		"summary initial\n" +
		"filename a.txt\n" +
		"\thello world\n"
	lines, err := parseBlamePorcelain(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("len = %d, want 1", len(lines))
	}
	got := lines[0]
	if got.Hash != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
		t.Errorf("hash = %q", got.Hash)
	}
	if got.ShortHash != "deadbee" {
		t.Errorf("short = %q, want deadbee", got.ShortHash)
	}
	if got.Author != "Alice Example" {
		t.Errorf("author = %q", got.Author)
	}
	if got.Content != "hello world" {
		t.Errorf("content = %q", got.Content)
	}
	if got.Time.Unix() != 1700000000 {
		t.Errorf("time = %v (unix %d), want 1700000000", got.Time, got.Time.Unix())
	}
}

func TestParseBlamePorcelain_MultipleLinesDifferentCommits(t *testing.T) {
	// Two lines from different commits; each has its own full header
	// block in line-porcelain output.
	in := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 1 1 2\n" +
		"author A\n" +
		"author-time 1000000000\n" +
		"summary a\n" +
		"filename f\n" +
		"\tline-one\n" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb 2 2 1\n" +
		"author B\n" +
		"author-time 2000000000\n" +
		"summary b\n" +
		"filename f\n" +
		"\tline-two\n"
	lines, err := parseBlamePorcelain(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("len = %d, want 2", len(lines))
	}
	if lines[0].Content != "line-one" || lines[0].Author != "A" || lines[0].ShortHash != "aaaaaaa" {
		t.Errorf("line 0: %+v", lines[0])
	}
	if lines[1].Content != "line-two" || lines[1].Author != "B" || lines[1].ShortHash != "bbbbbbb" {
		t.Errorf("line 1: %+v", lines[1])
	}
}

func TestParseBlamePorcelain_ContentStartingWithTab(t *testing.T) {
	// If the file line itself starts with a tab, the porcelain row
	// begins with two tabs; we only strip the first.
	in := "cccccccccccccccccccccccccccccccccccccccc 1 1 1\n" +
		"author C\n" +
		"author-time 1000000000\n" +
		"summary c\n" +
		"filename f\n" +
		"\t\tindented\n"
	lines, err := parseBlamePorcelain(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("len = %d, want 1", len(lines))
	}
	if lines[0].Content != "\tindented" {
		t.Errorf("content = %q, want leading tab preserved", lines[0].Content)
	}
}

func TestParseBlameHashLine(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOk bool
	}{
		{"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef 1 1 1", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", true},
		{"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef 1 1", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", true},
		{"author Somebody", "", false},
		{"short 1 1", "", false},
		{"NOTHEXxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx 1 1", "", false},
	}
	for _, c := range cases {
		got, ok := parseBlameHashLine(c.in)
		if got != c.want || ok != c.wantOk {
			t.Errorf("%q: got (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.wantOk)
		}
	}
}

func TestBlameAt_EndToEnd(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "a.txt", "one\ntwo\n", "first")
	commit(t, dir, "a.txt", "one\nTWO\nthree\n", "second")

	commits, err := History(dir, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	// Newest commit has 3 lines: "one" (from first), "TWO" and
	// "three" (from second).
	blame, err := BlameAt(dir, commits[0].Hash, "a.txt")
	if err != nil {
		t.Fatalf("blame: %v", err)
	}
	if len(blame) != 3 {
		t.Fatalf("len = %d, want 3", len(blame))
	}
	// Commits[0] is the second commit; commits[1] is the first.
	first := commits[1].Hash
	second := commits[0].Hash
	want := []struct {
		content string
		hash    string
	}{
		{"one", first},
		{"TWO", second},
		{"three", second},
	}
	for i, w := range want {
		if blame[i].Content != w.content {
			t.Errorf("line %d content = %q, want %q", i, blame[i].Content, w.content)
		}
		if blame[i].Hash != w.hash {
			t.Errorf("line %d hash = %q, want %q", i, blame[i].Hash, w.hash)
		}
		if blame[i].Author != "test" {
			t.Errorf("line %d author = %q, want test", i, blame[i].Author)
		}
		if blame[i].Time.IsZero() {
			t.Errorf("line %d time is zero", i)
		}
		// Sanity: commit time within the last hour.
		if time.Since(blame[i].Time) > time.Hour {
			t.Errorf("line %d time is suspiciously old: %v", i, blame[i].Time)
		}
	}
}

func TestSource_Blame(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "a.txt", "alpha\n", "first")
	src, err := NewSource(dir + "/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	commits := src.Commits()
	blame, err := src.Blame(commits[0].Hash)
	if err != nil {
		t.Fatalf("blame: %v", err)
	}
	if len(blame) != 1 {
		t.Fatalf("len = %d, want 1", len(blame))
	}
	if !strings.HasPrefix(blame[0].Content, "alpha") {
		t.Errorf("content = %q", blame[0].Content)
	}
}
