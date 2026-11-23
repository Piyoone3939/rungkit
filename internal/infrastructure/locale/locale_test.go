package locale

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"RUNGKIT_LANG": "ja", "LANG": "en_US.UTF-8"}, "ja"},
		{map[string]string{"RUNGKIT_LANG": "en", "LANG": "ja_JP.UTF-8"}, "en"},
		{map[string]string{"LANG": "ja_JP.UTF-8"}, "ja"},
		{map[string]string{"LC_ALL": "en_GB.UTF-8", "LANG": "ja_JP.UTF-8"}, "en"},
		{map[string]string{"LANG": "fr_FR.UTF-8"}, "en"},
	}
	for _, c := range cases {
		for _, k := range []string{"RUNGKIT_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
			t.Setenv(k, c.env[k])
		}
		if got := Detect(); got != c.want {
			t.Errorf("%v: got %s want %s", c.env, got, c.want)
		}
	}
}
