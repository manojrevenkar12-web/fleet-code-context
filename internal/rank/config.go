package rank

import (
	"encoding/json"
	"os"
)

// LoadSettings reads config/rank.json; missing keys keep their defaults.
func LoadSettings(path string) (Settings, error) {
	s := Defaults
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(b, &s)
}
