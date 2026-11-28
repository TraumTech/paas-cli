package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TRA-555/AC-4, AC-5
func TestPlatformEnv(t *testing.T) {
	cases := map[string]struct {
		stapel, paas, want string
	}{
		"только STAPEL_":        {stapel: "new", want: "new"},
		"только прежняя PAAS_":  {paas: "old", want: "old"},
		"обе — главнее STAPEL_": {stapel: "new", paas: "old", want: "new"},
		"ни одной":              {},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("STAPEL_API_URL", tc.stapel)
			t.Setenv("PAAS_API_URL", tc.paas)
			require.Equal(t, tc.want, platformEnv(envAPIURL))
			require.Equal(t, tc.want, envOr(envAPIURL, ""))
		})
	}
}
