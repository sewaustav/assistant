package platform

import (
	"reflect"
	"testing"
)

func TestParseCombo(t *testing.T) {
	cases := []struct {
		in   string
		mods []string
		key  string
	}{
		{"ctrl+c", []string{"ctrl"}, "c"},
		{"ctrl+shift+t", []string{"ctrl", "shift"}, "t"},
		{"alt+F4", []string{"alt"}, "f4"},
		{"win+d", []string{"super"}, "d"},
		{"Delete", nil, "delete"},
		{"volumeup", nil, "volumeup"},
		{"medianext", nil, "medianext"},
		{"enter", nil, "enter"},
	}
	for _, c := range cases {
		got := parseCombo(c.in)
		if !reflect.DeepEqual(got.mods, c.mods) || got.key != c.key {
			t.Errorf("parseCombo(%q) = %+v, want mods=%v key=%q", c.in, got, c.mods, c.key)
		}
	}
}
