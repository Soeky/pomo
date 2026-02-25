package stats

import "testing"

func TestFormatMinutesToHM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   int
		want string
	}{
		{in: 0, want: "00:00"},
		{in: 5, want: "00:05"},
		{in: 65, want: "01:05"},
		{in: 150, want: "02:30"},
	}

	for _, tt := range tests {
		got := FormatMinutesToHM(tt.in)
		if got != tt.want {
			t.Fatalf("FormatMinutesToHM(%d): got=%q want=%q", tt.in, got, tt.want)
		}
	}
}

func TestFormatRangeName(t *testing.T) {
	t.Parallel()

	if got := FormatRangeName("all"); got != "All Time" {
		t.Fatalf("unexpected all label: %q", got)
	}
	if got := FormatRangeName("custom"); got != "custom" {
		t.Fatalf("unexpected passthrough label: %q", got)
	}
}
