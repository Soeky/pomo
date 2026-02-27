package topics

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		want      Path
		expectErr bool
	}{
		{
			name:  "empty defaults",
			input: "",
			want:  Path{Domain: "General", Subtopic: "General"},
		},
		{
			name:  "domain only",
			input: "Math",
			want:  Path{Domain: "Math", Subtopic: "General"},
		},
		{
			name:  "domain and subtopic",
			input: "Math::Discrete Probability",
			want:  Path{Domain: "Math", Subtopic: "Discrete Probability"},
		},
		{
			name:  "trim spaces",
			input: "  Applied Math  ::  Numerical Analysis  ",
			want:  Path{Domain: "Applied Math", Subtopic: "Numerical Analysis"},
		},
		{
			name:      "missing domain",
			input:     "::Discrete",
			expectErr: true,
		},
		{
			name:  "missing subtopic defaults",
			input: "Math::",
			want:  Path{Domain: "Math", Subtopic: "General"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Parse(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected path: got=%+v want=%+v", got, tt.want)
			}
			if got.Canonical() != tt.want.Canonical() {
				t.Fatalf("unexpected canonical: got=%s want=%s", got.Canonical(), tt.want.Canonical())
			}
		})
	}
}
