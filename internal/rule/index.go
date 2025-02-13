//file: internal/rule/index.go
package rule

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/metrics"
)

// RuleIndex provides fast rule lookup with wildcard support
type RuleIndex struct {
	exactMatches map[string][]*Rule            // For exact topic matches
	wildcardTree *TopicTree                    // For wildcard topic patterns
	stats        IndexStats                    // Index statistics
	logger       *logger.Logger                // Logger instance
	metrics      *metrics.Metrics              // Metrics collector
	mu           sync.RWMutex                  // Protects index updates
}

// IndexStats tracks rule index statistics
type IndexStats struct {
	RuleCount     uint64    // Total number of rules
	Lookups       uint64    // Number of topic lookups
	Matches       uint64    // Number of successful matches
	LastUpdate    time.Time // Last index update time
	LastError     error     // Last error encountered
	WildcardRules uint64    // Number of wildcard rules
}

// NewRuleIndex creates a new rule index
func NewRuleIndex(logger *logger.Logger, metrics *metrics.Metrics) *RuleIndex {
	return &RuleIndex{
		exactMatches: make(map[string][]*Rule),
		wildcardTree: &TopicTree{
			root: &TopicNode{
				children: make(map[string]*TopicNode),
			},
		},
		logger:  logger,
		metrics: metrics,
		stats: IndexStats{
			LastUpdate: time.Now(),
		},
	}
}

// Add adds a rule to the index
func (idx *RuleIndex) Add(rule *Rule) error {
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Check if topic contains wildcards
	if containsWildcard(rule.Topic) {
		if err := idx.wildcardTree.AddRule(rule); err != nil {
			idx.logger.Error("failed to add wildcard rule",
				"topic", rule.Topic,
				"error", err)
			idx.stats.LastError = err
			return err
		}
		atomic.AddUint64(&idx.stats.WildcardRules, 1)
	} else {
		// Add rule to exact matches
		idx.exactMatches[rule.Topic] = append(idx.exactMatches[rule.Topic], rule)
	}

	atomic.AddUint64(&idx.stats.RuleCount, 1)
	idx.stats.LastUpdate = time.Now()

	if idx.metrics != nil {
		idx.metrics.SetRulesActive(float64(atomic.LoadUint64(&idx.stats.RuleCount)))
	}

	idx.logger.Debug("rule added to index",
		"topic", rule.Topic,
		"isWildcard", containsWildcard(rule.Topic))

	return nil
}

// Remove removes a rule from the index
func (idx *RuleIndex) Remove(rule *Rule) error {
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	if containsWildcard(rule.Topic) {
		if err := idx.wildcardTree.RemoveRule(rule); err != nil {
			idx.logger.Error("failed to remove wildcard rule",
				"topic", rule.Topic,
				"error", err)
			idx.stats.LastError = err
			return err
		}
		atomic.AddUint64(&idx.stats.WildcardRules, ^uint64(0))
	} else {
		rules := idx.exactMatches[rule.Topic]
		for i, r := range rules {
			if r == rule {
				idx.exactMatches[rule.Topic] = append(rules[:i], rules[i+1:]...)
				break
			}
		}
		if len(idx.exactMatches[rule.Topic]) == 0 {
			delete(idx.exactMatches, rule.Topic)
		}
	}

	atomic.AddUint64(&idx.stats.RuleCount, ^uint64(0))
	idx.stats.LastUpdate = time.Now()

	if idx.metrics != nil {
		idx.metrics.SetRulesActive(float64(atomic.LoadUint64(&idx.stats.RuleCount)))
	}

	idx.logger.Debug("rule removed from index",
		"topic", rule.Topic,
		"isWildcard", containsWildcard(rule.Topic))

	return nil
}

// Find finds all rules matching a topic
func (idx *RuleIndex) Find(topic string) []*Rule {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	atomic.AddUint64(&idx.stats.Lookups, 1)

	var matches []*Rule

	// Check exact matches first
	if exactMatches, ok := idx.exactMatches[topic]; ok {
		matches = append(matches, exactMatches...)
	}

	// Check wildcard matches
	if wildcardMatches := idx.wildcardTree.FindMatches(topic); len(wildcardMatches) > 0 {
		matches = append(matches, wildcardMatches...)
	}

	if len(matches) > 0 {
		atomic.AddUint64(&idx.stats.Matches, 1)
		if idx.metrics != nil {
			idx.metrics.IncRuleMatches()
		}
	}

	idx.logger.Debug("rule lookup completed",
		"topic", topic,
		"matchCount", len(matches))

	return matches
}

// Clear removes all rules from the index
func (idx *RuleIndex) Clear() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.exactMatches = make(map[string][]*Rule)
	idx.wildcardTree = &TopicTree{
		root: &TopicNode{
			children: make(map[string]*TopicNode),
		},
	}

	atomic.StoreUint64(&idx.stats.RuleCount, 0)
	atomic.StoreUint64(&idx.stats.WildcardRules, 0)
	idx.stats.LastUpdate = time.Now()

	if idx.metrics != nil {
		idx.metrics.SetRulesActive(0)
	}

	idx.logger.Info("rule index cleared")
}

// GetStats returns current index statistics
func (idx *RuleIndex) GetStats() IndexStats {
	return IndexStats{
		RuleCount:     atomic.LoadUint64(&idx.stats.RuleCount),
		Lookups:       atomic.LoadUint64(&idx.stats.Lookups),
		Matches:       atomic.LoadUint64(&idx.stats.Matches),
		WildcardRules: atomic.LoadUint64(&idx.stats.WildcardRules),
		LastUpdate:    idx.stats.LastUpdate,
		LastError:     idx.stats.LastError,
	}
}

// containsWildcard checks if a topic contains MQTT wildcards
func containsWildcard(topic string) bool {
	return strings.Contains(topic, "+") || strings.Contains(topic, "#")
}
