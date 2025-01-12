// File: internal/rule/loader.go
package rule

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func LoadRules(path string) ([]Rule, error) {
	var rules []Rule

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var ruleSet []Rule
		if err := json.Unmarshal(data, &ruleSet); err != nil {
			return err
		}

		rules = append(rules, ruleSet...)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return rules, nil
}
