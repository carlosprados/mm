package schedule

import (
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	t.Run("relative duration", func(t *testing.T) {
		got, err := ParseTime("+2h")
		if err != nil {
			t.Fatal(err)
		}
		if d := time.Until(got); d < 119*time.Minute || d > 121*time.Minute {
			t.Errorf("+2h resolved to %v from now", d)
		}
	})

	t.Run("RFC3339", func(t *testing.T) {
		got, err := ParseTime("2030-01-02T15:04:05Z")
		if err != nil {
			t.Fatal(err)
		}
		if want := time.Date(2030, 1, 2, 15, 4, 5, 0, time.UTC); !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("local datetime", func(t *testing.T) {
		got, err := ParseTime("2030-01-02 15:04")
		if err != nil {
			t.Fatal(err)
		}
		if got.Year() != 2030 || got.Hour() != 15 || got.Minute() != 4 {
			t.Errorf("unexpected parse: %v", got)
		}
	})

	t.Run("errors", func(t *testing.T) {
		for _, in := range []string{"", "not a time", "+notdur"} {
			if _, err := ParseTime(in); err == nil {
				t.Errorf("expected error for %q", in)
			}
		}
	})
}
