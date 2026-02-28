package cmd

import "testing"

func TestResolveWebModeValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		flagMode      string
		daemonFlag    bool
		daemonChanged bool
		configMode    string
		want          string
		wantErr       bool
	}{
		{
			name:       "mode flag wins",
			flagMode:   "on_demand",
			configMode: "daemon",
			want:       webModeOnDemand,
		},
		{
			name:          "daemon compatibility override true",
			daemonFlag:    true,
			daemonChanged: true,
			configMode:    "on_demand",
			want:          webModeDaemon,
		},
		{
			name:          "daemon compatibility override false",
			daemonFlag:    false,
			daemonChanged: true,
			configMode:    "daemon",
			want:          webModeOnDemand,
		},
		{
			name:       "config fallback",
			configMode: "on_demand",
			want:       webModeOnDemand,
		},
		{
			name: "empty config defaults daemon",
			want: webModeDaemon,
		},
		{
			name:     "invalid mode from flag",
			flagMode: "invalid",
			wantErr:  true,
		},
		{
			name:       "invalid mode from config",
			configMode: "invalid",
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveWebModeValue(tc.flagMode, tc.daemonFlag, tc.daemonChanged, tc.configMode)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got mode %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveWebModeValue error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected mode: got=%s want=%s", got, tc.want)
			}
		})
	}
}
