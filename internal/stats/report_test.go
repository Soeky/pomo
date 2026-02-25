package stats

import (
	"strings"
	"testing"
	"time"
)

func TestResolveRange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		args      []string
		wantStart string
		wantEnd   string
		wantLabel string
	}{
		{
			name:      "single day",
			args:      []string{"2026-02-24"},
			wantStart: "2026-02-24",
			wantEnd:   "2026-02-25",
			wantLabel: "2026-02-24",
		},
		{
			name:      "single month",
			args:      []string{"2026-02"},
			wantStart: "2026-02-01",
			wantEnd:   "2026-03-01",
			wantLabel: "2026-02",
		},
		{
			name:      "single year",
			args:      []string{"2026"},
			wantStart: "2026-01-01",
			wantEnd:   "2027-01-01",
			wantLabel: "2026",
		},
		{
			name:      "range",
			args:      []string{"2026-02-01", "2026-02-10"},
			wantStart: "2026-02-01",
			wantEnd:   "2026-02-11",
			wantLabel: "2026-02-01 – 2026-02-10",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			start, end, label := resolveRange(tt.args, now)
			if start.Format("2006-01-02") != tt.wantStart {
				t.Fatalf("unexpected start: got=%s want=%s", start.Format("2006-01-02"), tt.wantStart)
			}
			if end.Format("2006-01-02") != tt.wantEnd {
				t.Fatalf("unexpected end: got=%s want=%s", end.Format("2006-01-02"), tt.wantEnd)
			}
			if label != tt.wantLabel {
				t.Fatalf("unexpected label: got=%q want=%q", label, tt.wantLabel)
			}
		})
	}
}

func TestRenderReport(t *testing.T) {
	t.Parallel()

	out := RenderReport(StatsReport{
		Label:        "2026-02-25",
		Work:         []WorkLine{{Topic: "A", Count: 2, Minutes: 50}},
		BreakCount:   1,
		BreakMinutes: 10,
		WorkTotalMin: 50,
		WorkAvgMin:   25.0,
		HasWorkAvg:   true,
		BreakAvgMin:  10.0,
		HasBreakAvg:  true,
	})

	needles := []string{
		"📅 2026-02-25",
		"🍅 Work:",
		"A",
		"💤 Break:",
		"🧠 Total:",
	}
	for _, n := range needles {
		if !strings.Contains(out, n) {
			t.Fatalf("render output missing %q", n)
		}
	}
}
