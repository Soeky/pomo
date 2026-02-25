package parse

import (
	"testing"
	"time"
)

func TestParseDurationFromArg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		arg     string
		want    time.Duration
		wantErr bool
	}{
		{name: "minutes", arg: "25m", want: 25 * time.Minute},
		{name: "mixed", arg: "1h30m15s", want: time.Hour + 30*time.Minute + 15*time.Second},
		{name: "upper case", arg: "2H5M", want: 2*time.Hour + 5*time.Minute},
		{name: "invalid empty", arg: "", wantErr: true},
		{name: "invalid garbage", arg: "abc", wantErr: true},
		{name: "invalid partial", arg: "10mfoo", wantErr: true},
		{name: "invalid separator", arg: "10m 5s", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseDurationFromArg(tt.arg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for arg=%q", tt.arg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected duration: got=%v want=%v", got, tt.want)
			}
		})
	}
}
