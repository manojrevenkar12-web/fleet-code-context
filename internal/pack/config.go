package pack

import (
	"encoding/json"
	"os"
)

// LoadOptions reads config/pack.json; missing keys keep their defaults.
func LoadOptions(path string) (Options, error) {
	o := DefaultOptions
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return o, nil
	}
	if err != nil {
		return o, err
	}
	var raw struct {
		Budget       *int     `json:"budget_tokens"`
		MaxRepos     *int     `json:"max_repos"`
		FileFloor    *float64 `json:"file_floor"`
		SnippetLines *int     `json:"snippet_lines"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return o, err
	}
	if raw.Budget != nil {
		o.Budget = *raw.Budget
	}
	if raw.MaxRepos != nil {
		o.MaxRepos = *raw.MaxRepos
	}
	if raw.FileFloor != nil {
		o.FileFloor = *raw.FileFloor
	}
	if raw.SnippetLines != nil {
		o.SnippetLines = *raw.SnippetLines
	}
	return o, nil
}
