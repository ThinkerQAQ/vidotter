package vidotter

import "testing"

func TestTunnelProcessOwnership(t *testing.T) {
	profile := "/home/test/.config/tunnel-client/vidotter.yaml"
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{"/usr/bin/tunnel-client run --config " + profile, true},
		{"/usr/bin/tunnel-client run --config /home/other/profile.yaml", false},
		{"/usr/bin/tunnel-client doctor --config " + profile, false},
		{"/usr/bin/other-service run --config " + profile, false},
	} {
		if got := isOwnTunnelCommandLine(tc.cmd, profile); got != tc.want {
			t.Errorf("running(%q)=%v, want %v", tc.cmd, got, tc.want)
		}
	}
}
