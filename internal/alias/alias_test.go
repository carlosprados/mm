package alias

import (
	"path/filepath"
	"testing"
)

func TestResolve(t *testing.T) {
	s := &Store{Aliases: map[string]string{
		"luis":    "luisdavid.francisco",
		"luisete": "luisdavid.francisco",
	}}

	cases := map[string]string{
		"luis":      "luisdavid.francisco", // alias
		"LUIS":      "luisdavid.francisco", // case-insensitive
		" luisete ": "luisdavid.francisco", // trims whitespace
		"marta":     "marta",               // unknown → passthrough
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
	if err := s.Add("Luis", "@luisdavid.francisco"); err != nil {
		t.Fatalf("valid add errored: %v", err)
	}
	// normalized lowercase key, stripped @ from username
	if s.Aliases["luis"] != "luisdavid.francisco" {
		t.Errorf("got %q", s.Aliases["luis"])
	}
}

func TestRemove(t *testing.T) {
	s := &Store{Aliases: map[string]string{"luis": "luisdavid.francisco"}}
	if err := s.Remove("nope"); err == nil {
		t.Error("removing unknown alias should error")
	}
	if err := s.Remove("LUIS"); err != nil {
		t.Errorf("remove should be case-insensitive: %v", err)
	}
	if _, ok := s.Aliases["luis"]; ok {
		t.Error("alias not removed")
	}
}

func TestAliasesFor(t *testing.T) {
	s := &Store{Aliases: map[string]string{
		"luis":    "luisdavid.francisco",
		"luisete": "luisdavid.francisco",
		"marta":   "marta.gomez",
	}}
	got := s.AliasesFor("luisdavid.francisco")
	if len(got) != 2 || got[0] != "luis" || got[1] != "luisete" {
		t.Errorf("AliasesFor = %v, want [luis luisete]", got)
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

	if err := s.Add("luis", "luisdavid.francisco"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Aliases["luis"] != "luisdavid.francisco" {
		t.Errorf("round-trip failed: %v", reloaded.Aliases)
	}
}
