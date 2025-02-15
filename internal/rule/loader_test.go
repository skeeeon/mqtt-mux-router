package rule

import (
	"testing"
	"os"
	"path/filepath"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"mqtt-mux-router/internal/logger"
)

// setupTestLogger creates a new logger instance for testing
func setupTestLogger(t *testing.T) *logger.Logger {
	zapLogger, err := zap.NewDevelopment()
	require.NoError(t, err)
	return &logger.Logger{Logger: zapLogger}
}

func TestNewRulesLoader(t *testing.T) {
	tests := []struct {
		name      string
		log       *logger.Logger
		wantError bool
	}{
		{
			name:      "valid logger",
			log:       setupTestLogger(t),
			wantError: false,
		},
		{
			name:      "nil logger",
			log:       nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewRulesLoader(tt.log)
			if tt.wantError {
				assert.Nil(t, loader, "loader should be nil with invalid logger")
			} else {
				assert.NotNil(t, loader, "loader should not be nil with valid logger")
				assert.Equal(t, tt.log, loader.logger, "logger should be properly assigned")
			}
		})
	}
}

func TestValidateRule(t *testing.T) {
	validRule := &Rule{
		Topic: "test/topic",
		Action: &Action{
			Topic:   "test/action",
			Payload: "test payload",
		},
	}

	validRuleWithConditions := &Rule{
		Topic: "test/topic",
		Conditions: &Conditions{
			Operator: "and",
			Items: []Condition{
				{
					Field:    "temperature",
					Operator: "gt",
					Value:    25.0,
				},
			},
		},
		Action: &Action{
			Topic:   "test/action",
			Payload: "test payload",
		},
	}

	tests := []struct {
		name      string
		rule      *Rule
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid simple rule",
			rule:      validRule,
			wantError: false,
		},
		{
			name:      "valid rule with conditions",
			rule:      validRuleWithConditions,
			wantError: false,
		},
		{
			name:      "nil rule",
			rule:      nil,
			wantError: true,
			errorMsg:  "rule cannot be nil",
		},
		{
			name: "empty topic",
			rule: &Rule{
				Topic: "",
				Action: &Action{
					Topic:   "test/action",
					Payload: "test payload",
				},
			},
			wantError: true,
			errorMsg:  "rule topic cannot be empty",
		},
		{
			name: "nil action",
			rule: &Rule{
				Topic:  "test/topic",
				Action: nil,
			},
			wantError: true,
			errorMsg:  "rule action cannot be nil",
		},
		{
			name: "empty action topic",
			rule: &Rule{
				Topic: "test/topic",
				Action: &Action{
					Topic:   "",
					Payload: "test payload",
				},
			},
			wantError: true,
			errorMsg:  "action topic cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRule(tt.rule)
			if tt.wantError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsValidOperator(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		want     bool
	}{
		{
			name:     "equals operator",
			operator: "eq",
			want:     true,
		},
		{
			name:     "not equals operator",
			operator: "neq",
			want:     true,
		},
		{
			name:     "greater than operator",
			operator: "gt",
			want:     true,
		},
		{
			name:     "less than operator",
			operator: "lt",
			want:     true,
		},
		{
			name:     "greater than or equal operator",
			operator: "gte",
			want:     true,
		},
		{
			name:     "less than or equal operator",
			operator: "lte",
			want:     true,
		},
		{
			name:     "exists operator",
			operator: "exists",
			want:     true,
		},
		{
			name:     "contains operator",
			operator: "contains",
			want:     true,
		},
		{
			name:     "invalid operator",
			operator: "invalid",
			want:     false,
		},
		{
			name:     "empty operator",
			operator: "",
			want:     false,
		},
		{
			name:     "case sensitive check - uppercase",
			operator: "EQ",
			want:     false,
		},
		{
			name:     "case sensitive check - mixed case",
			operator: "Eq",
			want:     false,
		},
		{
			name:     "similar but invalid operator",
			operator: "equal",
			want:     false,
		},
		{
			name:     "operator with whitespace",
			operator: " eq ",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidOperator(tt.operator)
			assert.Equal(t, tt.want, got, "isValidOperator(%q) = %v, want %v", tt.operator, got, tt.want)
		})
	}
}

func TestValidateConditions(t *testing.T) {
	tests := []struct {
		name        string
		conditions  *Conditions
		wantError   bool
		errorMsg    string
	}{
		{
			name: "valid simple AND condition",
			conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: "gt",
						Value:    25.0,
					},
					{
						Field:    "humidity",
						Operator: "lt",
						Value:    80.0,
					},
				},
			},
			wantError: false,
		},
		{
			name: "valid simple OR condition",
			conditions: &Conditions{
				Operator: "or",
				Items: []Condition{
					{
						Field:    "status",
						Operator: "eq",
						Value:    "active",
					},
					{
						Field:    "power",
						Operator: "gt",
						Value:    50,
					},
				},
			},
			wantError: false,
		},
		{
			name: "valid nested conditions",
			conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: "gt",
						Value:    25.0,
					},
				},
				Groups: []Conditions{
					{
						Operator: "or",
						Items: []Condition{
							{
								Field:    "humidity",
								Operator: "lt",
								Value:    80.0,
							},
							{
								Field:    "pressure",
								Operator: "gt",
								Value:    1000.0,
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name:       "nil conditions",
			conditions: nil,
			wantError:  true,
			errorMsg:   "conditions cannot be nil",
		},
		{
			name: "invalid operator",
			conditions: &Conditions{
				Operator: "invalid",
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: "gt",
						Value:    25.0,
					},
				},
			},
			wantError: true,
			errorMsg:  "invalid operator: invalid",
		},
		{
			name: "empty field name",
			conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{
						Field:    "",
						Operator: "gt",
						Value:    25.0,
					},
				},
			},
			wantError: true,
			errorMsg:  "condition field cannot be empty",
		},
		{
			name: "invalid condition operator",
			conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: "invalid",
						Value:    25.0,
					},
				},
			},
			wantError: true,
			errorMsg:  "invalid condition operator: invalid",
		},
		{
			name: "invalid nested group operator",
			conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: "gt",
						Value:    25.0,
					},
				},
				Groups: []Conditions{
					{
						Operator: "invalid",
						Items: []Condition{
							{
								Field:    "humidity",
								Operator: "lt",
								Value:    80.0,
							},
						},
					},
				},
			},
			wantError: true,
			errorMsg:  "invalid nested condition group: invalid operator: invalid",
		},
		{
			name: "deep nested valid conditions",
			conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{
						Field:    "level1",
						Operator: "eq",
						Value:    1,
					},
				},
				Groups: []Conditions{
					{
						Operator: "or",
						Items: []Condition{
							{
								Field:    "level2",
								Operator: "eq",
								Value:    2,
							},
						},
						Groups: []Conditions{
							{
								Operator: "and",
								Items: []Condition{
									{
										Field:    "level3",
										Operator: "eq",
										Value:    3,
									},
								},
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "empty conditions but valid operator",
			conditions: &Conditions{
				Operator: "and",
			},
			wantError: false,
		},
		{
			name: "multiple invalid conditions",
			conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{
						Field:    "",
						Operator: "invalid1",
						Value:    25.0,
					},
					{
						Field:    "temperature",
						Operator: "invalid2",
						Value:    30.0,
					},
				},
			},
			wantError: true,
			errorMsg:  "condition field cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConditions(tt.conditions)
			if tt.wantError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadFromDirectory(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "rules-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Setup test logger
	log := setupTestLogger(t)
	loader := NewRulesLoader(log)
	require.NotNil(t, loader)

	// Helper function to create test files
	createTestFile := func(name, content string) string {
		path := filepath.Join(tmpDir, name)
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
		return path
	}

	// Create test directories
	validDir := filepath.Join(tmpDir, "valid")
	require.NoError(t, os.Mkdir(validDir, 0755))

	// Create valid test rule files
	validRule1 := `[
		{
			"topic": "sensors/temperature",
			"action": {
				"topic": "alerts/temperature",
				"payload": "high temperature"
			}
		}
	]`
	createTestFile(filepath.Join("valid", "valid1.json"), validRule1)

	validRule2 := `[
		{
			"topic": "sensors/humidity",
			"conditions": {
				"operator": "and",
				"items": [
					{
						"field": "humidity",
						"operator": "gt",
						"value": 80
					}
				]
			},
			"action": {
				"topic": "alerts/humidity",
				"payload": "high humidity"
			}
		}
	]`
	createTestFile(filepath.Join("valid", "valid2.json"), validRule2)

	// Create a subdirectory with rules
	subDir := filepath.Join(validDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755))
	createTestFile(filepath.Join("valid", "subdir", "valid3.json"), validRule1)

	tests := []struct {
		name      string
		setup     func() string
		validate  func(t *testing.T, rules []Rule)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid directory with valid rules",
			setup: func() string {
				return validDir
			},
			validate: func(t *testing.T, rules []Rule) {
				assert.Len(t, rules, 3) // Two valid files plus one in subdir
				// Verify rule contents
				topics := make([]string, len(rules))
				for i, rule := range rules {
					topics[i] = rule.Topic
				}
				assert.Contains(t, topics, "sensors/temperature")
				assert.Contains(t, topics, "sensors/humidity")
			},
		},
		{
			name: "non-existent directory",
			setup: func() string {
				return filepath.Join(tmpDir, "nonexistent")
			},
			wantErr: true,
			errMsg:  "failed to load rules",
		},
		{
			name: "directory with no permissions",
			setup: func() string {
				noPermDir := filepath.Join(tmpDir, "noperm")
				require.NoError(t, os.Mkdir(noPermDir, 0000))
				return noPermDir
			},
			wantErr: true,
			errMsg:  "permission denied",
		},
		{
			name: "empty directory",
			setup: func() string {
				emptyDir := filepath.Join(tmpDir, "empty")
				require.NoError(t, os.Mkdir(emptyDir, 0755))
				return emptyDir
			},
			validate: func(t *testing.T, rules []Rule) {
				assert.Empty(t, rules)
			},
		},
		{
			name: "directory with only invalid files",
			setup: func() string {
				invalidDir := filepath.Join(tmpDir, "invalid")
				require.NoError(t, os.Mkdir(invalidDir, 0755))

				invalidJSON := `{
					"this": "is not valid rule JSON",
					"missing": "closing bracket"
				`
				invalidRule := `[
					{
						"topic": "",
						"action": {
							"topic": "",
							"payload": ""
						}
					}
				]`

				err := os.WriteFile(filepath.Join(invalidDir, "invalid1.json"), []byte(invalidJSON), 0644)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(invalidDir, "invalid2.json"), []byte(invalidRule), 0644)
				require.NoError(t, err)

				return invalidDir
			},
			wantErr: true,
			errMsg:  "failed to load rules",
		},
		{
			name: "mixed valid and non-json files",
			setup: func() string {
				mixedDir := filepath.Join(tmpDir, "mixed")
				require.NoError(t, os.Mkdir(mixedDir, 0755))

				// Create a valid JSON file
				err := os.WriteFile(filepath.Join(mixedDir, "valid.json"), []byte(validRule1), 0644)
				require.NoError(t, err)

				// Create a non-JSON file
				err = os.WriteFile(filepath.Join(mixedDir, "notjson.txt"), []byte("This is not a JSON file"), 0644)
				require.NoError(t, err)

				return mixedDir
			},
			validate: func(t *testing.T, rules []Rule) {
				assert.Len(t, rules, 1) // Should only load the valid JSON file
				assert.Equal(t, "sensors/temperature", rules[0].Topic)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			rules, err := loader.LoadFromDirectory(path)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, rules)
				}
			}
		})
	}
}

func TestLoadFromDirectory_AdditionalCases(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "rules-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Setup test logger
	log := setupTestLogger(t)
	loader := NewRulesLoader(log)
	require.NotNil(t, loader)

	tests := []struct {
		name      string
		setup     func() string
		validate  func(t *testing.T, rules []Rule)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "unreadable file",
			setup: func() string {
				dir := filepath.Join(tmpDir, "unreadable")
				require.NoError(t, os.Mkdir(dir, 0755))

				filePath := filepath.Join(dir, "test.json")
				require.NoError(t, os.WriteFile(filePath, []byte("{}"), 0644))
				require.NoError(t, os.Chmod(filePath, 0000))

				return dir
			},
			wantErr: true,
			errMsg:  "failed to read rule file",
		},
		{
			name: "malformed json array",
			setup: func() string {
				dir := filepath.Join(tmpDir, "malformed")
				require.NoError(t, os.Mkdir(dir, 0755))

				content := `[{"topic": "test"} this is not valid json]`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "test.json"), []byte(content), 0644))

				return dir
			},
			wantErr: true,
			errMsg:  "failed to parse rule file",
		},
		{
			name: "valid json but not rule array",
			setup: func() string {
				dir := filepath.Join(tmpDir, "invalid-structure")
				require.NoError(t, os.Mkdir(dir, 0755))

				content := `{"this": "is valid json", "but": "not a rule array"}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "test.json"), []byte(content), 0644))

				return dir
			},
			wantErr: true,
			errMsg:  "failed to parse rule file",
		},
		{
			name: "rule with invalid condition structure",
			setup: func() string {
				dir := filepath.Join(tmpDir, "invalid-condition")
				require.NoError(t, os.Mkdir(dir, 0755))

				content := `[{
					"topic": "test/topic",
					"conditions": {
						"operator": "invalid",
						"items": [
							{
								"field": "test",
								"operator": "eq",
								"value": 123
							}
						]
					},
					"action": {
						"topic": "test/action",
						"payload": "test"
					}
				}]`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "test.json"), []byte(content), 0644))

				return dir
			},
			wantErr: true,
			errMsg:  "invalid operator",
		},
		{
			name: "symlink to valid file",
			setup: func() string {
				dir := filepath.Join(tmpDir, "symlink")
				require.NoError(t, os.Mkdir(dir, 0755))

				// Create valid file
				validContent := `[{
					"topic": "test/topic",
					"action": {
						"topic": "test/action",
						"payload": "test"
					}
				}]`
				validPath := filepath.Join(dir, "valid.json")
				require.NoError(t, os.WriteFile(validPath, []byte(validContent), 0644))

				// Create symlink
				linkPath := filepath.Join(dir, "link.json")
				require.NoError(t, os.Symlink(validPath, linkPath))

				return dir
			},
			validate: func(t *testing.T, rules []Rule) {
				assert.Len(t, rules, 2) // Should load both the original and symlinked file
				for _, rule := range rules {
					assert.Equal(t, "test/topic", rule.Topic)
				}
			},
		},
		{
			name: "broken symlink",
			setup: func() string {
				dir := filepath.Join(tmpDir, "broken-symlink")
				require.NoError(t, os.Mkdir(dir, 0755))

				// Create symlink to non-existent file
				linkPath := filepath.Join(dir, "broken.json")
				require.NoError(t, os.Symlink(filepath.Join(dir, "nonexistent.json"), linkPath))

				return dir
			},
			wantErr: true,
			errMsg:  "failed to read rule file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			rules, err := loader.LoadFromDirectory(path)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, rules)
				}
			}
		})
	}
}

func TestValidateRule_AdditionalCases(t *testing.T) {
	tests := []struct {
		name      string
		rule      *Rule
		wantError bool
		errorMsg  string
	}{
		{
			name: "deeply nested conditions",
			rule: &Rule{
				Topic: "test/topic",
				Conditions: &Conditions{
					Operator: "and",
					Items: []Condition{
						{Field: "test", Operator: "eq", Value: 1},
					},
					Groups: []Conditions{
						{
							Operator: "or",
							Groups: []Conditions{
								{
									Operator: "and",
									Items: []Condition{
										{Field: "nested", Operator: "eq", Value: 2},
									},
								},
							},
						},
					},
				},
				Action: &Action{
					Topic:   "test/action",
					Payload: "test",
				},
			},
			wantError: false,
		},
		{
			name: "empty conditions group",
			rule: &Rule{
				Topic: "test/topic",
				Conditions: &Conditions{
					Operator: "and",
					Items:    []Condition{},
					Groups:   []Conditions{},
				},
				Action: &Action{
					Topic:   "test/action",
					Payload: "test",
				},
			},
			wantError: false,
		},
		{
			name: "condition with nil value",
			rule: &Rule{
				Topic: "test/topic",
				Conditions: &Conditions{
					Operator: "and",
					Items: []Condition{
						{Field: "test", Operator: "exists", Value: nil},
					},
				},
				Action: &Action{
					Topic:   "test/action",
					Payload: "test",
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRule(tt.rule)
			if tt.wantError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
