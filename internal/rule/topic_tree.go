//file: internal/rule/topic_tree.go

package rule

import (
	"fmt"
	"strings"
	"sync"
)

// TopicTree implements a prefix tree for efficient wildcard topic matching
type TopicTree struct {
	root *TopicNode
	mu   sync.RWMutex
}

// TopicNode represents a node in the topic tree
type TopicNode struct {
	segment    string
	isWildcard bool
	rules      []*Rule
	children   map[string]*TopicNode
}

// AddRule adds a rule to the tree
func (t *TopicTree) AddRule(rule *Rule) error {
	if rule == nil || rule.Topic == "" {
		return fmt.Errorf("invalid rule or empty topic")
	}

	segments := strings.Split(rule.Topic, "/")
	
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
			// Add rule to leaf node
			next.rules = append(next.rules, rule)
		}
		current = next
	}

	return nil
}

// RemoveRule removes a rule from the tree
func (t *TopicTree) RemoveRule(rule *Rule) error {
	if rule == nil || rule.Topic == "" {
		return fmt.Errorf("invalid rule or empty topic")
	}

	segments := strings.Split(rule.Topic, "/")

	t.mu.Lock()
	defer t.mu.Unlock()

	return t.removeRule(t.root, segments, 0, rule)
}

// removeRule recursively removes a rule from the tree
func (t *TopicTree) removeRule(node *TopicNode, segments []string, depth int, rule *Rule) error {
	if node == nil || depth >= len(segments) {
		return nil
	}

	segment := segments[depth]
	child, exists := node.children[segment]
	if !exists {
		return fmt.Errorf("topic segment not found: %s", segment)
	}

	if depth == len(segments)-1 {
		// Remove rule from leaf node
		for i, r := range child.rules {
			if r == rule {
				child.rules = append(child.rules[:i], child.rules[i+1:]...)
				break
			}
		}
		// Remove node if it has no rules and no children
		if len(child.rules) == 0 && len(child.children) == 0 {
			delete(node.children, segment)
		}
		return nil
	}

	// Recurse deeper
	err := t.removeRule(child, segments, depth+1, rule)
	if err != nil {
		return err
	}

	// Clean up empty branches
	if len(child.rules) == 0 && len(child.children) == 0 {
		delete(node.children, segment)
	}

	return nil
}

// FindMatches finds all rules matching a topic
func (t *TopicTree) FindMatches(topic string) []*Rule {
	if topic == "" {
		return nil
	}

	segments := strings.Split(topic, "/")

	t.mu.RLock()
	defer t.mu.RUnlock()

	var matches []*Rule
	t.findMatches(t.root, segments, 0, &matches)
	return matches
}

// findMatches recursively finds matching rules
func (t *TopicTree) findMatches(node *TopicNode, segments []string, depth int, matches *[]*Rule) {
	if node == nil {
		return
	}

	// If we've matched all segments, collect rules
	if depth == len(segments) {
		if len(node.rules) > 0 {
			*matches = append(*matches, node.rules...)
		}
		return
	}

	segment := segments[depth]
	nextDepth := depth + 1

	// Check exact match
	if child, ok := node.children[segment]; ok {
		t.findMatches(child, segments, nextDepth, matches)
	}

	// Check single-level wildcard
	if child, ok := node.children["+"]; ok {
		t.findMatches(child, segments, nextDepth, matches)
	}

	// Check multi-level wildcard
	if child, ok := node.children["#"]; ok {
		*matches = append(*matches, child.rules...)
	}
}
