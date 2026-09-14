package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run(nil, &stdout, &stderr)
	if got != 2 {
		t.Errorf("run(nil) = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "Usage") {
		t.Errorf("run(nil) stderr = %q, want it to mention the usage", stderr.String())
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"-h"}, &stdout, &stderr)
	if got != 0 {
		t.Errorf("run(-h) = %d, want 0", got)
	}
	if !strings.Contains(stdout.String(), "Usage") {
		t.Errorf("run(-h) stdout = %q, want it to mention the usage", stdout.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"bogus"}, &stdout, &stderr)
	if got != 2 {
		t.Errorf("run(bogus) = %d, want 2", got)
	}
}

func TestRunAnalyseNoWords(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"analyse"}, &stdout, &stderr)
	if got != 2 {
		t.Errorf("run(analyse) = %d, want 2", got)
	}
}

func TestRunAnalyseWord(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"analyse", "chat"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("run(analyse chat) = %d, stderr = %q", got, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"Masquées", "Muettes", "ch", "chatte"} {
		if !strings.Contains(out, want) {
			t.Errorf("run(analyse chat) stdout = %q, want it to contain %q", out, want)
		}
	}
}

func TestRunAnalyseSentence(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"analyse", "les", "chats", "dorment"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("run(analyse les chats dorment) = %d, stderr = %q", got, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"Accordées", "pluriel_s"} {
		if !strings.Contains(out, want) {
			t.Errorf("run(analyse les chats dorment) stdout = %q, want it to contain %q", out, want)
		}
	}
}

func TestRunAnalyseJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"analyse", "-json", "chat"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("run(analyse -json chat) = %d, stderr = %q", got, stderr.String())
	}
	var words []wordReport
	if err := json.Unmarshal(stdout.Bytes(), &words); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stdout.String(), err)
	}
	if len(words) != 1 {
		t.Fatalf("len(words) = %d, want 1", len(words))
	}
	if words[0].Word != "chat" {
		t.Errorf("words[0].Word = %q, want %q", words[0].Word, "chat")
	}
	if words[0].Colors["Masquées"] != 1 {
		t.Errorf("words[0].Colors[Masquées] = %d, want 1", words[0].Colors["Masquées"])
	}
}

func TestRunAnalyseUnknownWord(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"analyse", "xyzzyblorp"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("run(analyse xyzzyblorp) = %d, stderr = %q", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "à vérifier") {
		t.Errorf("run(analyse xyzzyblorp) stdout = %q, want it to mention « à vérifier »", stdout.String())
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := run([]string{"version"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("run(version) = %d, stderr = %q", got, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Error("run(version) stdout is empty, want a version string")
	}
}
