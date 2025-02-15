package rule

import (
	"fmt"
	"sort"
	"sync"
	"testing"

	"go.uber.org/zap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"mqtt-mux-router/internal/logger"
)

func setupTestIndex(t *testing.T) *RuleIndex {
	zapLogger, err := zap.NewDevelopment()
	require.NoError(t, err)
	log := &logger.Logger{Logger: zapLogger}
	return NewRuleIndex(log)
}

func TestNewRuleIndex(t *testing.T) {
	idx := setupTestIndex(t)
	assert.NotNil(t, idx)
	assert.NotNil(t, idx.exactMatches)
	assert.NotNil(t, idx.logger)
	
	// Verify initial state
	assert.Empty(t, idx.exactMatches)
	stats := idx.GetStats()
	assert.Equal(t, uint64(0), stats.lookups)
	assert.Equal(t, uint64(0), stats.matches)
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name     string
		setupFn  func(*RuleIndex) *Rule // Setup function to prepare test state
		validate func(*testing.T, *RuleIndex, *Rule)
		wantLen  int
	}{
		{
			name: "add first rule",
			setupFn: func(idx *RuleIndex) *Rule {
				return &Rule{
					Topic: "sensors/temperature",
					Action: &Action{
						Topic:   "alerts",
						Payload: "high temperature",
					},
				}
			},
			wantLen: 1,
			validate: func(t *testing.T, idx *RuleIndex, rule *Rule) {
				rules := idx.exactMatches[rule.Topic]
				assert.Equal(t, 1, len(rules), "should have exactly one rule")
				assert.Equal(t, rule, rules[0], "stored rule should match input")
				assert.Equal(t, rule.Action.Payload, rules[0].Action.Payload, "action payload should match")
			},
		},
		{
			name: "add second rule same topic",
			setupFn: func(idx *RuleIndex) *Rule {
				// Add first rule
				firstRule := &Rule{
					Topic: "sensors/temperature",
					Action: &Action{
						Topic:   "alerts",
						Payload: "first alert",
					},
				}
				idx.Add(firstRule)

				// Return second rule to be added
				return &Rule{
					Topic: "sensors/temperature",
					Action: &Action{
						Topic:   "alerts",
						Payload: "second alert",
					},
				}
			},
			wantLen: 2,
			validate: func(t *testing.T, idx *RuleIndex, rule *Rule) {
				rules := idx.exactMatches[rule.Topic]
				assert.Equal(t, 2, len(rules), "should have exactly two rules")
				assert.Contains(t, rules, rule, "should contain the second rule")
				
				// Verify both payloads exist
				payloads := []string{rules[0].Action.Payload, rules[1].Action.Payload}
				assert.Contains(t, payloads, "first alert", "should contain first alert payload")
				assert.Contains(t, payloads, "second alert", "should contain second alert payload")
			},
		},
		{
			name: "add rule different topic",
			setupFn: func(idx *RuleIndex) *Rule {
				// Add a rule to existing topic
				existingRule := &Rule{
					Topic: "sensors/temperature",
					Action: &Action{
						Topic:   "alerts",
						Payload: "existing alert",
					},
				}
				idx.Add(existingRule)

				// Return new rule with different topic
				return &Rule{
					Topic: "sensors/humidity",
					Action: &Action{
						Topic:   "alerts",
						Payload: "humidity alert",
					},
				}
			},
			wantLen: 1,
			validate: func(t *testing.T, idx *RuleIndex, rule *Rule) {
				// Verify the new topic has exactly one rule
				rules := idx.exactMatches[rule.Topic]
				assert.Equal(t, 1, len(rules), "should have exactly one rule for new topic")
				assert.Equal(t, rule, rules[0], "stored rule should match input")
				
				// Verify existing topic still has its rule
				existingRules := idx.exactMatches["sensors/temperature"]
				assert.Equal(t, 1, len(existingRules), "existing topic should maintain its rule")
			},
		},
		{
			name: "add nil rule",
			setupFn: func(idx *RuleIndex) *Rule {
				return nil
			},
			wantLen: 0,
			validate: func(t *testing.T, idx *RuleIndex, rule *Rule) {
				totalRules := 0
				for _, rules := range idx.exactMatches {
					totalRules += len(rules)
				}
				assert.Equal(t, 0, totalRules, "index should not contain any rules")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new index for each test to ensure isolation
			testIdx := setupTestIndex(t)
			prevStats := testIdx.GetStats()
			
			// Setup test state and get rule to add
			rule := tt.setupFn(testIdx)
			
			// Add the rule (if not nil)
			if rule != nil {
				testIdx.Add(rule)
				rules := testIdx.exactMatches[rule.Topic]
				assert.Equal(t, tt.wantLen, len(rules))
			}
			
			// Run test-specific validation
			tt.validate(t, testIdx, rule)
			
			// Verify stats were updated appropriately
			stats := testIdx.GetStats()
			if rule != nil {
				assert.True(t, stats.lastUpdated.After(prevStats.lastUpdated))
			}
		})
	}
}

func TestFind(t *testing.T) {
	idx := setupTestIndex(t)

	// Add test rules with distinguishable payloads
	rule1 := &Rule{
		Topic: "test/topic1",
		Action: &Action{
			Topic:   "alerts",
			Payload: "payload1",
		},
	}
	rule2 := &Rule{
		Topic: "test/topic1",
		Action: &Action{
			Topic:   "alerts",
			Payload: "payload2",
		},
	}
	rule3 := &Rule{
		Topic: "test/topic2",
		Action: &Action{
			Topic:   "alerts",
			Payload: "payload3",
		},
	}

	idx.Add(rule1)
	idx.Add(rule2)
	idx.Add(rule3)

	tests := []struct {
		name      string
		topic     string
		wantCount int
		validate  func(*testing.T, []*Rule)
	}{
		{
			name:      "find existing topic with multiple rules",
			topic:     "test/topic1",
			wantCount: 2,
			validate: func(t *testing.T, rules []*Rule) {
				assert.Contains(t, []string{rules[0].Action.Payload, rules[1].Action.Payload}, "payload1")
				assert.Contains(t, []string{rules[0].Action.Payload, rules[1].Action.Payload}, "payload2")
			},
		},
		{
			name:      "find existing topic with single rule",
			topic:     "test/topic2",
			wantCount: 1,
			validate: func(t *testing.T, rules []*Rule) {
				assert.Equal(t, "payload3", rules[0].Action.Payload)
			},
		},
		{
			name:      "find non-existent topic",
			topic:     "test/nonexistent",
			wantCount: 0,
			validate: func(t *testing.T, rules []*Rule) {
				assert.Empty(t, rules)
			},
		},
		{
			name:      "find with empty topic",
			topic:     "",
			wantCount: 0,
			validate: func(t *testing.T, rules []*Rule) {
				assert.Empty(t, rules)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prevStats := idx.GetStats()
			rules := idx.Find(tt.topic)
			assert.Equal(t, tt.wantCount, len(rules))
			tt.validate(t, rules)
			
			// Verify stats were updated
			stats := idx.GetStats()
			assert.Equal(t, prevStats.lookups+1, stats.lookups)
			if tt.wantCount > 0 {
				assert.Equal(t, prevStats.matches+1, stats.matches)
			}
		})
	}
}

func TestGetTopics(t *testing.T) {
	idx := setupTestIndex(t)

	// Test empty index
	topics := idx.GetTopics()
	assert.Empty(t, topics)

	// Add rules in non-alphabetical order
	rules := []*Rule{
		{Topic: "test/topicC"},
		{Topic: "test/topicC"}, // Duplicate topic
		{Topic: "test/topicA"},
		{Topic: "test/topicB"},
	}

	for _, rule := range rules {
		idx.Add(rule)
	}

	// Test getting topics
	topics = idx.GetTopics()
	assert.Equal(t, 3, len(topics))
	
	// Verify topics are returned in a consistent order
	expected := []string{"test/topicA", "test/topicB", "test/topicC"}
	sort.Strings(topics)
	assert.Equal(t, expected, topics)
}

func TestClear(t *testing.T) {
	idx := setupTestIndex(t)

	// Add some rules
	rules := []*Rule{
		{Topic: "test/topic1"},
		{Topic: "test/topic2"},
		{Topic: "test/topic3"},
	}

	for _, rule := range rules {
		idx.Add(rule)
	}

	// Verify rules were added
	assert.Equal(t, 3, len(idx.GetTopics()))

	preStats := idx.GetStats()
	idx.Clear()

	// Verify index is empty
	assert.Empty(t, idx.GetTopics())
	assert.Empty(t, idx.exactMatches)

	// Verify stats reset
	postStats := idx.GetStats()
	assert.True(t, postStats.lastUpdated.After(preStats.lastUpdated))
}

func TestGetStats(t *testing.T) {
	idx := setupTestIndex(t)

	// Initial stats should be zero
	initial := idx.GetStats()
	assert.Equal(t, uint64(0), initial.lookups)
	assert.Equal(t, uint64(0), initial.matches)

	// Add some rules and perform lookups
	rule := &Rule{Topic: "test/topic"}
	idx.Add(rule)
	
	// Test lookups with and without matches
	idx.Find("test/topic")  // Match
	idx.Find("test/topic")  // Match
	idx.Find("nonexistent") // No match

	// Get final stats
	stats := idx.GetStats()
	
	assert.Equal(t, uint64(3), stats.lookups)
	assert.Equal(t, uint64(2), stats.matches)
	assert.True(t, stats.lastUpdated.After(initial.lastUpdated))
}

func TestConcurrentAccess(t *testing.T) {
	idx := setupTestIndex(t)
	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2) // Writers and readers

	// Writers
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				rule := &Rule{
					Topic: fmt.Sprintf("test/topic%d", n),
					Action: &Action{
						Topic:   "alerts",
						Payload: fmt.Sprintf("test%d", j),
					},
				}
				idx.Add(rule)
			}
		}(i)
	}

	// Readers
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				topic := fmt.Sprintf("test/topic%d", n)
				_ = idx.Find(topic)
				_ = idx.GetTopics()
			}
		}(i)
	}

	wg.Wait()

	// After all goroutines complete, verify the index state
	topics := idx.GetTopics()
	assert.LessOrEqual(t, len(topics), goroutines)  // Should have at most one topic per writer goroutine

	// Verify we can still use the index
	testTopic := "test/topic0"
	rules := idx.Find(testTopic)
	assert.NotEmpty(t, rules)         // Should find rules for the first topic
	assert.Equal(t, testTopic, rules[0].Topic)  // Topic should match

	// Verify we can still add and find new rules
	newRule := &Rule{
		Topic: "test/final",
		Action: &Action{
			Topic:   "alerts",
			Payload: "final",
		},
	}
	idx.Add(newRule)
	foundRules := idx.Find("test/final")
	assert.Len(t, foundRules, 1)
	assert.Equal(t, newRule.Topic, foundRules[0].Topic)
}
