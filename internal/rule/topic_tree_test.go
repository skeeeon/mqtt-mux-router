//file: internal/rule/topic_tree_test.go
package rule

import (
	"sync"
	"testing"
	"time"
)

func TestTopicTreeBasicOperations(t *testing.T) {
	tree := &TopicTree{
		root: &TopicNode{
			children: make(map[string]*TopicNode),
		},
	}

	// Test cases for basic rule addition
	basicTests := []struct {
		name    string
		rule    *Rule
		topic   string
		wantErr bool
	}{
		{
			name: "Simple topic",
			rule: &Rule{
				Topic: "sensors/temperature",
				Action: &Action{
					Topic:        "alerts/temperature",
					TargetBroker: "broker1",
				},
			},
			topic:   "sensors/temperature",
			wantErr: false,
		},
		{
			name: "Single-level wildcard",
			rule: &Rule{
				Topic: "sensors/+/temperature",
				Action: &Action{
					Topic:        "alerts/temperature",
					TargetBroker: "broker1",
				},
			},
			topic:   "sensors/room1/temperature",
			wantErr: false,
		},
		{
			name: "Multi-level wildcard",
			rule: &Rule{
				Topic: "sensors/#",
				Action: &Action{
					Topic:        "alerts/all",
					TargetBroker: "broker1",
				},
			},
			topic:   "sensors/room1/temperature/celsius",
			wantErr: false,
		},
		{
			name: "Invalid multi-level wildcard placement",
			rule: &Rule{
				Topic: "sensors/#/temperature",
				Action: &Action{
					Topic:        "alerts/invalid",
					TargetBroker: "broker1",
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid single-level wildcard",
			rule: &Rule{
				Topic: "sensors/+temp/data",
				Action: &Action{
					Topic:        "alerts/invalid",
					TargetBroker: "broker1",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			err := tree.AddRule(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.topic != "" {
				matches := tree.FindMatches(tt.topic)
				if len(matches) == 0 {
					t.Errorf("FindMatches() found no matches for topic %s", tt.topic)
				}
			}
		})
	}
}

func TestTopicTreeMatchingPatterns(t *testing.T) {
	tree := &TopicTree{
		root: &TopicNode{
			children: make(map[string]*TopicNode),
		},
	}

	// Setup test rules
	rules := []*Rule{
		{
			Topic: "sensors/+/temperature",
			Action: &Action{
				Topic:        "alerts/temperature",
				TargetBroker: "broker1",
			},
		},
		{
			Topic: "sensors/room1/#",
			Action: &Action{
				Topic:        "alerts/room1",
				TargetBroker: "broker1",
			},
		},
		{
			Topic: "sensors/room1/temperature",
			Action: &Action{
				Topic:        "alerts/room1/temp",
				TargetBroker: "broker1",
			},
		},
	}

	// Add all rules
	for _, rule := range rules {
		if err := tree.AddRule(rule); err != nil {
			t.Fatalf("Failed to add rule: %v", err)
		}
	}

	// Test cases
	tests := []struct {
		name          string
		topic         string
		wantMatches   int
		wantTopics    []string
	}{
		{
			name:        "Exact match",
			topic:       "sensors/room1/temperature",
			wantMatches: 3, // Matches all rules
			wantTopics:  []string{"sensors/+/temperature", "sensors/room1/#", "sensors/room1/temperature"},
		},
		{
			name:        "Single-level wildcard match",
			topic:       "sensors/room2/temperature",
			wantMatches: 1,
			wantTopics:  []string{"sensors/+/temperature"},
		},
		{
			name:        "Multi-level wildcard match",
			topic:       "sensors/room1/temperature/celsius",
			wantMatches: 1,
			wantTopics:  []string{"sensors/room1/#"},
		},
		{
			name:        "No matches",
			topic:       "sensors/room2/humidity",
			wantMatches: 0,
			wantTopics:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := tree.FindMatches(tt.topic)
			if len(matches) != tt.wantMatches {
				t.Errorf("FindMatches() got %d matches, want %d", len(matches), tt.wantMatches)
			}

			// Verify matching topics
			matchedTopics := make([]string, 0, len(matches))
			for _, match := range matches {
				matchedTopics = append(matchedTopics, match.Topic)
			}

			if len(matchedTopics) != len(tt.wantTopics) {
				t.Errorf("FindMatches() got topics %v, want %v", matchedTopics, tt.wantTopics)
			}

			// Check each wanted topic is present
			for _, wantTopic := range tt.wantTopics {
				found := false
				for _, gotTopic := range matchedTopics {
					if gotTopic == wantTopic {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("FindMatches() missing topic %s", wantTopic)
				}
			}
		})
	}
}

func TestTopicTreeConcurrentOperations(t *testing.T) {
	tree := &TopicTree{
		root: &TopicNode{
			children: make(map[string]*TopicNode),
		},
	}

	// Number of concurrent operations
	const numOperations = 1000
	var wg sync.WaitGroup

	// Add rules concurrently
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rule := &Rule{
				Topic: "sensors/concurrent",
				Action: &Action{
					Topic:        "alerts/concurrent",
					TargetBroker: "broker1",
				},
			}
			_ = tree.AddRule(rule)
		}(i)
	}

	// Find matches concurrently
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = tree.FindMatches("sensors/concurrent")
		}()
	}

	// Remove rules concurrently
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rule := &Rule{
				Topic: "sensors/concurrent",
				Action: &Action{
					Topic:        "alerts/concurrent",
					TargetBroker: "broker1",
				},
			}
			_ = tree.RemoveRule(rule)
		}(i)
	}

	// Use channel and timer to detect deadlocks
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible deadlock")
	}
}

func TestTopicTreeEdgeCases(t *testing.T) {
	tree := &TopicTree{
		root: &TopicNode{
			children: make(map[string]*TopicNode),
		},
	}

	tests := []struct {
		name    string
		rule    *Rule
		wantErr bool
	}{
		{
			name: "Empty topic",
			rule: &Rule{
				Topic: "",
				Action: &Action{
					Topic:        "alerts/empty",
					TargetBroker: "broker1",
				},
			},
			wantErr: true,
		},
		{
			name:    "Nil rule",
			rule:    nil,
			wantErr: true,
		},
		{
			name: "Leading slash",
			rule: &Rule{
				Topic: "/sensors/temperature",
				Action: &Action{
					Topic:        "alerts/temp",
					TargetBroker: "broker1",
				},
			},
			wantErr: false,
		},
		{
			name: "Trailing slash",
			rule: &Rule{
				Topic: "sensors/temperature/",
				Action: &Action{
					Topic:        "alerts/temp",
					TargetBroker: "broker1",
				},
			},
			wantErr: false,
		},
		{
			name: "Double slash",
			rule: &Rule{
				Topic: "sensors//temperature",
				Action: &Action{
					Topic:        "alerts/temp",
					TargetBroker: "broker1",
				},
			},
			wantErr: false, // Empty segments are allowed per validateTopic implementation
		},
		{
			name: "Invalid wildcard combination",
			rule: &Rule{
				Topic: "sensors/+/#/temperature",
				Action: &Action{
					Topic:        "alerts/temp",
					TargetBroker: "broker1",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tree.AddRule(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Test FindMatches with edge cases
	findTests := []struct {
		name    string
		topic   string
		wantLen int
	}{
		{
			name:    "Empty topic",
			topic:   "",
			wantLen: 0,
		},
		{
			name:    "Single slash",
			topic:   "/",
			wantLen: 0,
		},
		{
			name:    "Very long topic",
			topic:   "a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p",
			wantLen: 0,
		},
	}

	for _, tt := range findTests {
		t.Run(tt.name, func(t *testing.T) {
			matches := tree.FindMatches(tt.topic)
			if len(matches) != tt.wantLen {
				t.Errorf("FindMatches() got %d matches, want %d", len(matches), tt.wantLen)
			}
		})
	}
}

func BenchmarkTopicTree(b *testing.B) {
	tree := &TopicTree{
		root: &TopicNode{
			children: make(map[string]*TopicNode),
		},
	}

	// Setup some rules for benchmarking
	rules := []*Rule{
		{Topic: "sensors/+/temperature"},
		{Topic: "sensors/room1/#"},
		{Topic: "sensors/room1/temperature"},
		{Topic: "devices/+/status"},
		{Topic: "devices/laptop/#"},
	}

	for _, rule := range rules {
		if err := tree.AddRule(rule); err != nil {
			b.Fatalf("Failed to add rule: %v", err)
		}
	}

	b.Run("AddRule", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rule := &Rule{
				Topic: "test/benchmark/add",
				Action: &Action{
					Topic:        "alerts/benchmark",
					TargetBroker: "broker1",
				},
			}
			_ = tree.AddRule(rule)
		}
	})

	b.Run("FindMatches/ExactMatch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = tree.FindMatches("sensors/room1/temperature")
		}
	})

	b.Run("FindMatches/WildcardMatch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = tree.FindMatches("sensors/room2/temperature")
		}
	})

	b.Run("FindMatches/NoMatch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = tree.FindMatches("not/matching/topic")
		}
	})

	b.Run("RemoveRule", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rule := &Rule{
				Topic: "test/benchmark/remove",
				Action: &Action{
					Topic:        "alerts/benchmark",
					TargetBroker: "broker1",
				},
			}
			_ = tree.AddRule(rule)
			_ = tree.RemoveRule(rule)
		}
	})
}
