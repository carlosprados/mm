package alias

import (
	"path/filepath"
	"testing"
)

func TestResolve(t *testing.T) {
	s := &Store{Aliases: map[string]string{
		"alex":   "alexandra.hernandez",
		"sandra": "alexandra.hernandez",
	}}

	cases := map[string]string{
		"alex":     "alexandra.hernandez", // alias
		"ALEX":     "alexandra.hernandez", // case-insensitive
		" sandra ": "alexandra.hernandez", // trims whitespace
		"marta":    "marta",               // unknown → passthrough
	}
	for in, want := range cases {
		if got := s.Resolve(in); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", in, got, want)
		}
	}

	if got := (*Store)(nil).Resolve("x"); got != "x" {
		t.Errorf("nil store Resolve should passthrough, got %q", got)
	}
}

func TestAddValidation(t *testing.T) {
	s := &Store{Aliases: map[string]string{}}

	if err := s.Add("", "user"); err == nil {
		t.Error("empty alias should error")
	}
	if err := s.Add("foo bar", "user"); err == nil {
		t.Error("alias with whitespace should error")
	}
	if err := s.Add("foo", ""); err == nil {
		t.Error("empty username should error")
	}
	if err := s.Add("Alex", "@alexandra.hernandez"); err != nil {
		t.Fatalf("valid add errored: %v", err)
	}
	// normalized lowercase key, stripped @ from username
	if s.Aliases["alex"] != "alexandra.hernandez" {
		t.Errorf("got %q", s.Aliases["alex"])
	}
}

func TestRemove(t *testing.T) {
	s := &Store{Aliases: map[string]string{"alex": "alexandra.hernandez"}}
	if err := s.Remove("nope"); err == nil {
		t.Error("removing unknown alias should error")
	}
	if err := s.Remove("ALEX"); err != nil {
		t.Errorf("remove should be case-insensitive: %v", err)
	}
	if _, ok := s.Aliases["alex"]; ok {
		t.Error("alias not removed")
	}
}

func TestAliasesFor(t *testing.T) {
	s := &Store{Aliases: map[string]string{
		"alex":   "alexandra.hernandez",
		"sandra": "alexandra.hernandez",
		"marta":  "marta.gomez",
	}}
	got := s.AliasesFor("alexandra.hernandez")
	if len(got) != 2 || got[0] != "alex" || got[1] != "sandra" {
		t.Errorf("AliasesFor = %v, want [alex sandra]", got)
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(dir, "mm", "aliases.json") {
		t.Errorf("unexpected path %q", p)
	}

	// Load with no file yet → empty store.
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Aliases) != 0 {
		t.Errorf("expected empty store, got %v", s.Aliases)
	}

	if err := s.Add("alex", "alexandra.hernandez"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Aliases["alex"] != "alexandra.hernandez" {
		t.Errorf("round-trip failed: %v", reloaded.Aliases)
	}
}
