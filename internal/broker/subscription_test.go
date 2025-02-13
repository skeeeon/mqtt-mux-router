//file: internal/broker/subscription_test.go

package broker

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestNewSubscriptionManager(t *testing.T) {
	sm := NewSubscriptionManager()
	if sm == nil {
		t.Fatal("Expected non-nil subscription manager")
	}
	if sm.topics == nil {
		t.Error("Expected non-nil topics map")
	}
	if sm.wildcards == nil {
		t.Error("Expected non-nil wildcards tree")
	}
	if sm.wildcards.root == nil {
		t.Error("Expected non-nil wildcards root")
	}
}

func TestSubscriptionValidation(t *testing.T) {
	tests := []struct {
		name      string
		topic     string
		isFilter  bool
		wantError bool
	}{
		// Valid subscription filters
		{"Valid simple topic", "sensors/temp", true, false},
		{"Valid single-level wildcard", "sensors/+/temp", true, false},
		{"Valid multi-level wildcard", "sensors/#", true, false},
		{"Valid complex filter", "home/+/living/+/temp", true, false},
		{"Valid leading slash", "/sensors/temp", true, false},
		{"Valid trailing slash", "sensors/temp/", true, false},

		// Invalid subscription filters
		{"Empty topic", "", true, true},
		{"Invalid + wildcard", "sensors/+temp/value", true, true},
		{"Invalid # wildcard", "sensors/#/value", true, true},
		{"Mid-topic #", "sensors/#/temp", true, true},
		{"Empty middle segment", "sensors//temp", true, true},

		// Valid publish topics
		{"Valid publish topic", "sensors/temp", false, false},
		{"Valid multi-segment", "home/floor1/living/temp", false, false},
		{"Valid publish leading slash", "/sensors/temp", false, false},
		{"Valid publish trailing slash", "sensors/temp/", false, false},

		// Invalid publish topics
		{"Empty publish topic", "", false, true},
		{"Publish with +", "sensors/+/temp", false, true},
		{"Publish with #", "sensors/#", false, true},
		{"Empty middle publish segment", "sensors//temp", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.isFilter {
				err = validateTopicFilter(tt.topic)
			} else {
				err = validateTopicName(tt.topic)
			}

			if (err != nil) != tt.wantError {
				t.Errorf("validation error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestSubscriptionManagerOperations(t *testing.T) {
	sm := NewSubscriptionManager()

	t.Run("Add and remove subscriptions", func(t *testing.T) {
		tests := []struct {
			topic string
			qos   byte
		}{
			{"sensors/temp", 0},
			{"home/+/temp", 1},
			{"devices/#", 2},
		}

		// Add subscriptions
		for _, tt := range tests {
			if err := sm.AddSubscription(tt.topic, tt.qos); err != nil {
				t.Errorf("AddSubscription(%s) error = %v", tt.topic, err)
			}
		}

		// Verify subscriptions
		subs := sm.GetSubscriptions()
		for _, tt := range tests {
			if qos, exists := subs[tt.topic]; !exists {
				t.Errorf("Subscription %s not found", tt.topic)
			} else if qos != tt.qos {
				t.Errorf("Subscription %s has QoS %d, want %d", tt.topic, qos, tt.qos)
			}
		}

		// Remove subscriptions
		for _, tt := range tests {
			if err := sm.RemoveSubscription(tt.topic); err != nil {
				t.Errorf("RemoveSubscription(%s) error = %v", tt.topic, err)
			}
		}

		// Verify all removed
		subs = sm.GetSubscriptions()
		if len(subs) != 0 {
			t.Errorf("Expected empty subscriptions, got %d", len(subs))
		}
	})

	t.Run("Clear subscriptions", func(t *testing.T) {
		// Add some subscriptions
		topics := []string{"test/1", "test/+", "test/#"}
		for _, topic := range topics {
			if err := sm.AddSubscription(topic, 0); err != nil {
				t.Errorf("AddSubscription(%s) error = %v", topic, err)
			}
		}

		// Clear all
		sm.Clear()

		// Verify empty
		subs := sm.GetSubscriptions()
		if len(subs) != 0 {
			t.Errorf("Expected empty subscriptions after Clear(), got %d", len(subs))
		}
	})
}

func TestWildcardMatching(t *testing.T) {
	sm := NewSubscriptionManager()

	// Add test subscriptions
	subscriptions := []struct {
		filter string
		qos    byte
	}{
		{"sport/tennis/player1", 0},
		{"sport/+/player1", 1},
		{"sport/#", 2},
		{"sport/tennis/+", 1},
		{"+/tennis/player1", 1},
		{"+/+/player1", 2},
		{"#", 2},
	}

	for _, sub := range subscriptions {
		if err := sm.AddSubscription(sub.filter, sub.qos); err != nil {
			t.Fatalf("Failed to add subscription %s: %v", sub.filter, err)
		}
	}

	tests := []struct {
		name          string
		topic         string
		wantMatches   []TopicMatch
		matchesCount  int
		includesMatch string
	}{
		{
			name:         "Exact match",
			topic:        "sport/tennis/player1",
			matchesCount: 7,
			includesMatch: "sport/tennis/player1",
		},
		{
			name:         "Single-level wildcard",
			topic:        "sport/tennis/player2",
			matchesCount: 3,
			includesMatch: "sport/tennis/+",
		},
		{
			name:         "Multi-level wildcard",
			topic:        "sport/tennis/player1/ranking",
			matchesCount: 2,
			includesMatch: "sport/#",
		},
		{
			name:         "Multiple wildcards",
			topic:        "other/tennis/player1",
			matchesCount: 3,
			includesMatch: "+/tennis/player1",
		},
		{
			name:         "Root wildcard only",
			topic:        "other/game/player2",
			matchesCount: 1,
			includesMatch: "#",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := sm.Match(tt.topic)

			if len(matches) != tt.matchesCount {
				t.Errorf("got %d matches, want %d", len(matches), tt.matchesCount)
				for _, m := range matches {
					t.Logf("match: %s (QoS %d)", m.Topic, m.QoS)
				}
			}

			found := false
			for _, match := range matches {
				if match.Topic == tt.includesMatch {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected to find match %s", tt.includesMatch)
			}
		})
	}
}

func TestConcurrentOperations(t *testing.T) {
	sm := NewSubscriptionManager()
	var wg sync.WaitGroup
	workers := 10
	iterations := 100

	// Test concurrent subscription operations
	wg.Add(workers * 3) // 3 types of workers

	// Adding subscriptions
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				topic := fmt.Sprintf("test/%d/topic/%d", id, j)
				err := sm.AddSubscription(topic, 0)
				if err != nil {
					t.Errorf("AddSubscription error: %v", err)
				}
			}
		}(i)
	}

	// Matching topics
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				topic := fmt.Sprintf("test/%d/topic/%d", id, j)
				matches := sm.Match(topic)
				if matches == nil {
					t.Error("Match returned nil")
				}
			}
		}(i)
	}

	// Removing subscriptions
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				topic := fmt.Sprintf("test/%d/topic/%d", id, j)
				err := sm.RemoveSubscription(topic)
				if err != nil {
					t.Errorf("RemoveSubscription error: %v", err)
				}
			}
		}(i)
	}

	// Use a channel to collect errors from goroutines
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out")
	}
}

func TestTopicTreeOperations(t *testing.T) {
	tree := &TopicTree{
		root: &TopicNode{
			children: make(map[string]*TopicNode),
		},
	}

	t.Run("Add and remove topics", func(t *testing.T) {
		topics := []struct {
			topic string
			qos   byte
		}{
			{"a/b/c", 0},
			{"a/+/c", 1},
			{"a/#", 2},
		}

		// Add topics
		for _, tt := range topics {
			err := tree.AddTopic(tt.topic, tt.qos)
			if err != nil {
				t.Errorf("AddTopic(%s) error = %v", tt.topic, err)
			}
		}

		// Verify topics exist
		subs := tree.GetSubscriptions()
		for _, tt := range topics {
			if qos, exists := subs[tt.topic]; !exists {
				t.Errorf("Topic %s not found", tt.topic)
			} else if qos != tt.qos {
				t.Errorf("Topic %s has QoS %d, want %d", tt.topic, qos, tt.qos)
			}
		}

		// Remove topics
		for _, tt := range topics {
			err := tree.RemoveTopic(tt.topic)
			if err != nil {
				t.Errorf("RemoveTopic(%s) error = %v", tt.topic, err)
			}
		}

		// Verify all removed
		subs = tree.GetSubscriptions()
		if len(subs) != 0 {
			t.Errorf("Expected empty subscriptions, got %d", len(subs))
		}
	})
}

func TestMatchOrdering(t *testing.T) {
	sm := NewSubscriptionManager()

	// Add subscriptions in random order
	subscriptions := []struct {
		topic string
		qos   byte
	}{
		{"a/+/c", 1},
		{"a/b/c", 0},
		{"#", 2},
		{"a/#", 1},
	}

	for _, sub := range subscriptions {
		if err := sm.AddSubscription(sub.topic, sub.qos); err != nil {
			t.Fatalf("AddSubscription(%s) error = %v", sub.topic, err)
		}
	}

	matches := sm.Match("a/b/c")

	// Verify exact matches come before wildcards
	if len(matches) == 0 {
		t.Fatal("Expected matches, got none")
	}

	// Find the index of the exact match
	exactMatchIndex := -1
	for i, match := range matches {
		if match.Topic == "a/b/c" {
			exactMatchIndex = i
			break
		}
	}

	if exactMatchIndex == -1 {
		t.Error("Exact match not found in results")
	} else if exactMatchIndex != 0 {
		t.Errorf("Exact match found at index %d, expected 0", exactMatchIndex)
	}
}

func TestSubscriptionQoS(t *testing.T) {
	sm := NewSubscriptionManager()

	subscriptions := []struct {
		topic string
		qos   byte
	}{
		{"test/topic", 0},
		{"test/+", 1},
		{"test/#", 2},
	}

	// Add subscriptions
	for _, sub := range subscriptions {
		err := sm.AddSubscription(sub.topic, sub.qos)
		if err != nil {
			t.Fatalf("AddSubscription(%s) error = %v", sub.topic, err)
		}
	}

	// Match and verify QoS levels
	matches := sm.Match("test/topic")
	
	// Sort matches by QoS to make comparison easier
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].QoS < matches[j].QoS
	})

	if len(matches) != 3 {
		t.Fatalf("Expected 3 matches, got %d", len(matches))
	}

	expectedQoS := []byte{0, 1, 2}
	for i, match := range matches {
		if match.QoS != expectedQoS[i] {
			t.Errorf("Match %d has QoS %d, want %d", i, match.QoS, expectedQoS[i])
		}
	}
}
