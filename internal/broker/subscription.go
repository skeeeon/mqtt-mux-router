//file: internal/broker/subscription.go

package broker

import (
	"fmt"
	"strings"
)

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		topics: make(map[string]byte),
		wildcards: &TopicTree{
			root: &TopicNode{
				children: make(map[string]*TopicNode),
			},
		},
	}
}

// AddSubscription adds a topic subscription with specified QoS
func (sm *SubscriptionManager) AddSubscription(topic string, qos byte) error {
	if err := validateTopicFilter(topic); err != nil {
		return fmt.Errorf("invalid topic filter: %w", err)
	}

	// Handle wildcard subscriptions
	if strings.Contains(topic, "+") || strings.Contains(topic, "#") {
		sm.mu.Lock()
		defer sm.mu.Unlock()
		return sm.wildcards.AddTopic(topic, qos)
	}

	// Handle exact topic match
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.topics[topic] = qos
	return nil
}

// RemoveSubscription removes a topic subscription
func (sm *SubscriptionManager) RemoveSubscription(topic string) error {
	if err := validateTopicFilter(topic); err != nil {
		return fmt.Errorf("invalid topic filter: %w", err)
	}

	// Handle wildcard subscriptions
	if strings.Contains(topic, "+") || strings.Contains(topic, "#") {
		sm.mu.Lock()
		defer sm.mu.Unlock()
		return sm.wildcards.RemoveTopic(topic)
	}

	// Handle exact topic match
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.topics, topic)
	return nil
}

// Match finds all subscriptions matching a topic
func (sm *SubscriptionManager) Match(topic string) []TopicMatch {
	if err := validateTopicName(topic); err != nil {
		return []TopicMatch{}
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	matches := make([]TopicMatch, 0)

	// Check exact matches first
	if qos, ok := sm.topics[topic]; ok {
		matches = append(matches, TopicMatch{
			Topic: topic,
			QoS:   qos,
		})
	}

	// Check wildcard matches
	wildcardMatches := sm.wildcards.Match(topic)
	if len(wildcardMatches) > 0 {
		matches = append(matches, wildcardMatches...)
	}

	return matches
}

// GetSubscriptions returns all active subscriptions
func (sm *SubscriptionManager) GetSubscriptions() map[string]byte {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Create a new map to avoid external modifications
	subs := make(map[string]byte, len(sm.topics))

	// Copy exact match subscriptions
	for topic, qos := range sm.topics {
		subs[topic] = qos
	}

	// Add wildcard subscriptions
	wildcardSubs := sm.wildcards.GetSubscriptions()
	for topic, qos := range wildcardSubs {
		subs[topic] = qos
	}

	return subs
}

// Clear removes all subscriptions
func (sm *SubscriptionManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Clear exact match topics
	sm.topics = make(map[string]byte)

	// Reset wildcard tree
	sm.wildcards = &TopicTree{
		root: &TopicNode{
			children: make(map[string]*TopicNode),
		},
	}
}

// AddTopic adds a topic to the topic tree
func (t *TopicTree) AddTopic(topic string, qos byte) error {
	segments := strings.Split(topic, "/")

	t.mu.Lock()
	defer t.mu.Unlock()

	current := t.root
	for i, segment := range segments {
		isLast := i == len(segments)-1

		// Validate wildcards
		if segment == "#" && !isLast {
			return fmt.Errorf("multi-level wildcard (#) must be the last segment")
		}
		if strings.Contains(segment, "+") && segment != "+" {
			return fmt.Errorf("single-level wildcard (+) must be the entire segment")
		}

		// Create or get next node
		next, exists := current.children[segment]
		if !exists {
			next = &TopicNode{
				segment:    segment,
				isWildcard: segment == "+" || segment == "#",
				children:   make(map[string]*TopicNode),
			}
			current.children[segment] = next
		}

		if isLast {
			next.isEnd = true
			next.qos = qos
		}
		current = next
	}

	return nil
}

// RemoveTopic removes a topic from the topic tree
func (t *TopicTree) RemoveTopic(topic string) error {
	segments := strings.Split(topic, "/")

	t.mu.Lock()
	defer t.mu.Unlock()

	return t.removeNode(t.root, segments, 0)
}

// removeNode recursively removes nodes from the topic tree
func (t *TopicTree) removeNode(node *TopicNode, segments []string, depth int) error {
	if node == nil || depth >= len(segments) {
		return nil
	}

	segment := segments[depth]
	child, exists := node.children[segment]
	if !exists {
		return fmt.Errorf("topic not found")
	}

	// Last segment
	if depth == len(segments)-1 {
		if !child.isEnd {
			return fmt.Errorf("topic not found")
		}
		child.isEnd = false
		// Remove node if it has no children and is not an endpoint
		if len(child.children) == 0 {
			delete(node.children, segment)
		}
		return nil
	}

	// Recurse deeper
	err := t.removeNode(child, segments, depth+1)
	if err != nil {
		return err
	}

	// Clean up empty branches
	if !child.isEnd && len(child.children) == 0 {
		delete(node.children, segment)
	}

	return nil
}

// Match finds all topics in the tree that match the given topic
func (t *TopicTree) Match(topic string) []TopicMatch {
	segments := strings.Split(topic, "/")

	t.mu.RLock()
	defer t.mu.RUnlock()

	matches := make([]TopicMatch, 0)
	t.matchNode(t.root, segments, 0, "", &matches)
	return matches
}

// matchNode recursively finds matching topics in the tree
func (t *TopicTree) matchNode(node *TopicNode, segments []string, depth int, currentPath string, matches *[]TopicMatch) {
	if node == nil || depth > len(segments) {
		return
	}

	// Check if we've reached a matching endpoint
	if depth == len(segments) && node.isEnd {
		*matches = append(*matches, TopicMatch{
			Topic: currentPath,
			QoS:   node.qos,
		})
		return
	}

	// Handle # wildcard
	if wildcard, ok := node.children["#"]; ok && wildcard.isEnd {
		*matches = append(*matches, TopicMatch{
			Topic: buildTopicPath(currentPath, "#"),
			QoS:   wildcard.qos,
		})
	}

	if depth == len(segments) {
		return
	}

	segment := segments[depth]
	nextDepth := depth + 1

	// Try exact match
	if child, ok := node.children[segment]; ok {
		t.matchNode(child, segments, nextDepth,
			buildTopicPath(currentPath, segment), matches)
	}

	// Try + wildcard match
	if child, ok := node.children["+"]; ok {
		t.matchNode(child, segments, nextDepth,
			buildTopicPath(currentPath, "+"), matches)
	}
}

// GetSubscriptions returns all subscriptions in the tree
func (t *TopicTree) GetSubscriptions() map[string]byte {
	t.mu.RLock()
	defer t.mu.RUnlock()

	subs := make(map[string]byte)
	t.collectSubscriptions(t.root, "", subs)
	return subs
}

// collectSubscriptions recursively collects all subscriptions
func (t *TopicTree) collectSubscriptions(node *TopicNode, path string, subs map[string]byte) {
	if node == nil {
		return
	}

	if node.isEnd {
		if path != "" {
			subs[path] = node.qos
		}
	}

	for segment, child := range node.children {
		newPath := buildTopicPath(path, segment)
		t.collectSubscriptions(child, newPath, subs)
	}
}

// buildTopicPath joins topic segments with proper delimiters
func buildTopicPath(currentPath, segment string) string {
	if currentPath == "" {
		return segment
	}
	return currentPath + "/" + segment
}

// validateTopicFilter validates a subscription topic filter
func validateTopicFilter(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	segments := strings.Split(topic, "/")
	for i, segment := range segments {
		// Allow empty segments for leading/trailing slashes
		if segment == "" && i != 0 && i != len(segments)-1 {
			return fmt.Errorf("empty segment not allowed in middle of topic")
		}

		if strings.Contains(segment, "#") {
			if segment != "#" {
				return fmt.Errorf("# wildcard must occupy entire segment")
			}
			if i != len(segments)-1 {
				return fmt.Errorf("# wildcard must be the last segment")
			}
		}

		if strings.Contains(segment, "+") {
			if segment != "+" {
				return fmt.Errorf("+ wildcard must occupy entire segment")
			}
		}
	}

	return nil
}

// validateTopicName validates a publish topic name
func validateTopicName(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	if strings.Contains(topic, "+") || strings.Contains(topic, "#") {
		return fmt.Errorf("wildcards not allowed in topic names")
	}

	segments := strings.Split(topic, "/")
	for i, segment := range segments {
		if segment == "" && i != 0 && i != len(segments)-1 {
			return fmt.Errorf("empty segment not allowed in middle of topic")
		}
	}

	return nil
}
