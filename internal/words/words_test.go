package words

import (
	"reflect"
	"testing"
)

func TestSplit(t *testing.T) {
	cases := map[string][]string{
		"GetEtaRequest":           {"eta", "request"},
		"eta_seconds":             {"eta", "second"},
		"TopicRiderAssigned":      {"topic", "rider", "assign"},
		"HTTPServer":              {"http", "server"},
		"refunds are refunded":    {"refund", "refund"},
		"ListAvailableRiders":     {"list", "available", "rider"},
		"fleet.example/proto/v1":  {"fleet", "example", "proto", "v1"},
	}
	for in, want := range cases {
		if got := Split(in); !reflect.DeepEqual(got, want) {
			t.Errorf("Split(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestTokens(t *testing.T) {
	if Tokens("") != 0 || Tokens("abcd") != 1 || Tokens("abcde") != 2 {
		t.Fatal("token estimate")
	}
}
