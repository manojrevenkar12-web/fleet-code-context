package rank

import (
	"reflect"
	"testing"
)

// The checked-in config must say exactly what the defaults say, so a run without config reproduces the report.
func TestConfigMatchesDefaults(t *testing.T) {
	s, err := LoadSettings("../../config/rank.json")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, Defaults) {
		t.Fatalf("config/rank.json %+v differs from Defaults %+v", s, Defaults)
	}
}
