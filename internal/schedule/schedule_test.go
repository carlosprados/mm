package schedule

import (
	"testing"
	"time"
)

func TestAddValidation(t *testing.T) {
	s := &Store{}
	future := time.Now().Add(time.Hour)

	if _, err := s.Add("", "lbl", "hi", future); err == nil {
		t.Error("empty channel should error")
	}
	if _, err := s.Add("ch", "lbl", "   ", future); err == nil {
		t.Error("blank message should error")
	}
	if _, err := s.Add("ch", "lbl", "hi", time.Now().Add(-time.Minute)); err == nil {
		t.Error("past time should error")
	}
	it, err := s.Add("ch", "lbl", "hi", future)
	if err != nil {
		t.Fatal(err)
	}
	if it.ID == "" || len(s.Items) != 1 {
		t.Errorf("add failed: %+v", s.Items)
	}
}

func TestDueAndRemove(t *testing.T) {
	now := time.Now()
	s := &Store{Items: []Item{
		{ID: "a", ChannelID: "c", Message: "past", At: now.Add(-time.Minute)},
		{ID: "b", ChannelID: "c", Message: "future", At: now.Add(time.Hour)},
	}}

	due := s.Due(now)
	if len(due) != 1 || due[0].ID != "a" {
		t.Fatalf("Due = %+v", due)
	}

	if err := s.Remove("a"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("a"); err == nil {
		t.Error("removing twice should error")
	}
	if len(s.Items) != 1 || s.Items[0].ID != "b" {
		t.Errorf("remove failed: %+v", s.Items)
	}
}

func TestSortedAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	s, err := Load() // no file yet
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := s.Add("c", "later", "b", now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("c", "sooner", "a", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	sorted := reloaded.Sorted()
	if len(sorted) != 2 || sorted[0].Label != "sooner" || sorted[1].Label != "later" {
		t.Errorf("sort/round-trip failed: %+v", sorted)
	}
}
