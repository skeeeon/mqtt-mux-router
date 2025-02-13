//file: internal/rule/loader_test.go
package rule

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mqtt-mux-router/config"
	"mqtt-mux-router/internal/logger"
)

func setupTestLoader(t *testing.T) (*RulesLoader, string) {
	t.Helper()

	log, err := logger.NewLogger(&config.LogConfig{
		Level:      "debug",
		OutputPath: "stdout",
		Encoding:   "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "rules-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	return NewRulesLoader(log), tmpDir
}

func createTestFile(t *testing.T, dir, filename string, content interface{}) {
	t.Helper()

	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal content: %v", err)
	}

	err = os.WriteFile(filepath.Join(dir, filename), data, 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
}

// TestLoadSingleRule updated
func TestLoadSingleRule(t *testing.T) {
	loader, tmpDir := setupTestLoader(t)
	defer os.RemoveAll(tmpDir)

	// Create test rule with all required fields
	rule := Rule{
		Topic:        "sensors/+/temperature",
		SourceBroker: "source1",
		Description:  "Temperature monitoring",
		Enabled:      true,
		Action: &Action{
			Topic:        "alerts/${device_id}",
			TargetBroker: "target1",
			Payload:      "${temperature}",
			QoS:         1,
			Retain:      false,
		},
	}

	data, err := json.Marshal(rule)
	if err != nil {
		t.Fatalf("Failed to marshal rule: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "single_rule.json"), data, 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	rules, err := loader.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	if len(rules) != 1 {
		t.Fatalf("LoadFromDirectory() got %v rules, want 1", len(rules))
	}

	got := rules[0]
	if got.Topic != "sensors/+/temperature" {
		t.Errorf("Rule.Topic = %v, want sensors/+/temperature", got.Topic)
	}
	if got.SourceBroker != "source1" {
		t.Errorf("Rule.SourceBroker = %v, want source1", got.SourceBroker)
	}
	if got.CreatedAt.IsZero() {
		t.Error("Rule.CreatedAt should not be zero")
	}
	if got.Action == nil {
		t.Error("Rule.Action should not be nil")
	}
	if got.Action != nil && got.Action.TargetBroker != "target1" {
		t.Errorf("Rule.Action.TargetBroker = %v, want target1", got.Action.TargetBroker)
	}
}

func TestLoadRuleSet(t *testing.T) {
	loader, tmpDir := setupTestLoader(t)
	defer os.RemoveAll(tmpDir)

	ruleSet := map[string]interface{}{
		"name":        "Test Rules",
		"description": "Test rule set",
		"version":     "1.0",
		"rules": []interface{}{
			map[string]interface{}{
				"topic":        "sensors/+/temperature",
				"sourceBroker": "source1",
				"description":  "Temperature alerts",
				"enabled":      true,
				"action": map[string]interface{}{
					"topic":        "alerts/${device_id}",
					"targetBroker": "target1",
					"payload":      "${temperature}",
					"qos":         1,
					"retain":      false,
				},
			},
			map[string]interface{}{
				"topic":        "sensors/+/humidity",
				"sourceBroker": "source1",
				"description":  "Humidity alerts",
				"enabled":      true,
				"action": map[string]interface{}{
					"topic":        "alerts/${device_id}/humidity",
					"targetBroker": "target1",
					"payload":      "${humidity}",
					"qos":         1,
					"retain":      false,
				},
			},
		},
	}

	createTestFile(t, tmpDir, "ruleset.json", ruleSet)

	rules, err := loader.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	if len(rules) != 2 {
		t.Fatalf("LoadFromDirectory() got %v rules, want 2", len(rules))
	}

	for _, rule := range rules {
		if rule.CreatedAt.IsZero() {
			t.Error("Rule.CreatedAt should not be zero")
		}
		if !rule.Enabled {
			t.Error("Rule.Enabled should be true")
		}
		if rule.Action == nil {
			t.Error("Rule.Action should not be nil")
		}
		if rule.Action.TargetBroker == "" {
			t.Error("Rule.Action.TargetBroker should not be empty")
		}
	}
}

func TestInvalidRules(t *testing.T) {
	loader, tmpDir := setupTestLoader(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		content  interface{}
		filename string
		wantErr  bool
	}{
		{
			name: "Missing topic",
			content: map[string]interface{}{
				"sourceBroker": "source1",
				"enabled":      true,
				"action": map[string]interface{}{
					"topic":        "alerts",
					"targetBroker": "target1",
				},
			},
			filename: "missing_topic.json",
			wantErr:  true,
		},
		{
			name: "Invalid topic pattern",
			content: map[string]interface{}{
				"topic":        "sensors/#/temperature",
				"sourceBroker": "source1",
				"enabled":      true,
				"action": map[string]interface{}{
					"topic":        "alerts",
					"targetBroker": "target1",
				},
			},
			filename: "invalid_topic.json",
			wantErr:  true,
		},
		{
			name: "Missing target broker",
			content: map[string]interface{}{
				"topic":        "sensors/temperature",
				"sourceBroker": "source1",
				"enabled":      true,
				"action": map[string]interface{}{
					"topic": "alerts/temp",
					"qos":   1,
				},
			},
			filename: "missing_target.json",
			wantErr:  true,
		},
		{
			name: "Invalid condition operator",
			content: map[string]interface{}{
				"topic":        "sensors/temperature",
				"sourceBroker": "source1",
				"enabled":      true,
				"conditions": map[string]interface{}{
					"operator": "invalid",
					"items": []interface{}{
						map[string]interface{}{
							"field":    "temperature",
							"operator": "gt",
							"value":    30,
						},
					},
				},
				"action": map[string]interface{}{
					"topic":        "alerts/temp",
					"targetBroker": "target1",
				},
			},
			filename: "invalid_operator.json",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createTestFile(t, tmpDir, tt.filename, tt.content)

			_, err := loader.LoadFromDirectory(tmpDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}

			os.Remove(filepath.Join(tmpDir, tt.filename))
		})
	}
}

func TestEmptyDirectory(t *testing.T) {
	loader, tmpDir := setupTestLoader(t)
	defer os.RemoveAll(tmpDir)

	rules, err := loader.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	if len(rules) != 0 {
		t.Errorf("LoadFromDirectory() got %v rules, want 0", len(rules))
	}
}

func TestInvalidDirectoryPath(t *testing.T) {
	loader, _ := setupTestLoader(t)

	_, err := loader.LoadFromDirectory("/nonexistent/path")
	if err == nil {
		t.Error("LoadFromDirectory() should return error for nonexistent directory")
	}
}

func TestDuplicateTopics(t *testing.T) {
	loader, tmpDir := setupTestLoader(t)
	defer os.RemoveAll(tmpDir)

	ruleSet := map[string]interface{}{
		"name":    "Test Rules",
		"version": "1.0",
		"rules": []interface{}{
			map[string]interface{}{
				"topic":        "sensors/+/temperature",
				"sourceBroker": "source1",
				"enabled":      true,
				"action": map[string]interface{}{
					"topic":        "alerts/${device_id}",
					"targetBroker": "target1",
					"qos":         1,
				},
			},
			map[string]interface{}{
				"topic":        "sensors/+/temperature",
				"sourceBroker": "source2",
				"enabled":      true,
				"action": map[string]interface{}{
					"topic":        "alerts/${device_id}",
					"targetBroker": "target1",
					"qos":         1,
				},
			},
		},
	}

	createTestFile(t, tmpDir, "duplicate_topics.json", ruleSet)

	_, err := loader.LoadFromDirectory(tmpDir)
	if err == nil {
		t.Error("LoadFromDirectory() should return error for duplicate topics")
	}
}

// TestNonJSONFiles updated
func TestNonJSONFiles(t *testing.T) {
	loader, tmpDir := setupTestLoader(t)
	defer os.RemoveAll(tmpDir)

	// Create a non-JSON file
	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("not a json file"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create a valid rule file
	rule := Rule{
		Topic:        "sensors/temperature",
		SourceBroker: "source1",
		Enabled:      true,
		Action: &Action{
			Topic:        "alerts/temp",
			TargetBroker: "target1",
			QoS:         1,
		},
	}

	data, err := json.Marshal(rule)
	if err != nil {
		t.Fatalf("Failed to marshal rule: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "valid_rule.json"), data, 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Test loading
	rules, err := loader.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	if len(rules) != 1 {
		t.Errorf("LoadFromDirectory() got %v rules, want 1", len(rules))
	}
}

func TestInvalidRuleSet(t *testing.T) {
	loader, tmpDir := setupTestLoader(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		content  interface{}
		filename string
		wantErr  bool
	}{
		{
			name: "Missing name",
			content: map[string]interface{}{
				"version": "1.0",
				"rules":   []interface{}{},
			},
			filename: "missing_name.json",
			wantErr:  true,
		},
		{
			name: "Missing version",
			content: map[string]interface{}{
				"name":  "Test Rules",
				"rules": []interface{}{},
			},
			filename: "missing_version.json",
			wantErr:  true,
		},
		{
			name: "Empty rules",
			content: map[string]interface{}{
				"name":    "Test Rules",
				"version": "1.0",
				"rules":   []interface{}{},
			},
			filename: "empty_rules.json",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createTestFile(t, tmpDir, tt.filename, tt.content)

			_, err := loader.LoadFromDirectory(tmpDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}

			os.Remove(filepath.Join(tmpDir, tt.filename))
		})
	}
}

func TestRuleSetCreatedAt(t *testing.T) {
	loader, tmpDir := setupTestLoader(t)
	defer os.RemoveAll(tmpDir)

	createdAt := time.Now().Add(-24 * time.Hour)

	ruleSet := map[string]interface{}{
		"name":        "Test Rules",
		"version":     "1.0",
		"createdAt":   createdAt.Format(time.RFC3339),
		"rules": []interface{}{
			map[string]interface{}{
				"topic":        "sensors/temperature",
				"sourceBroker": "source1",
				"enabled":      true,
				"action": map[string]interface{}{
					"topic":        "alerts/temp",
					"targetBroker": "target1",
					"qos":         1,
				},
			},
		},
	}

	createTestFile(t, tmpDir, "ruleset.json", ruleSet)

	rules, err := loader.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	if len(rules) != 1 {
		t.Fatalf("LoadFromDirectory() got %v rules, want 1", len(rules))
	}

	if rules[0].CreatedAt.IsZero() {
		t.Error("Rule.CreatedAt should not be zero")
	}
}
