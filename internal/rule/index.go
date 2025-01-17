package rule

import (
    "sync"
    "sync/atomic"
    "time"
)

// RuleIndex provides fast topic-based rule lookup
type RuleIndex struct {
    exactMatches map[string][]*Rule
    mu           sync.RWMutex
    stats        IndexStats
}

// IndexStats tracks performance metrics
type IndexStats struct {
    lookups     uint64
    matches     uint64
    lastUpdated time.Time
    mu          sync.RWMutex
}

// NewRuleIndex creates a new rule index
func NewRuleIndex() *RuleIndex {
    return &RuleIndex{
        exactMatches: make(map[string][]*Rule),
        stats: IndexStats{
            lastUpdated: time.Now(),
        },
    }
}

// Add indexes a rule by its topic
func (idx *RuleIndex) Add(rule *Rule) {
    idx.mu.Lock()
    defer idx.mu.Unlock()

    idx.exactMatches[rule.Topic] = append(idx.exactMatches[rule.Topic], rule)
    idx.stats.lastUpdated = time.Now()
}

// Find returns all rules matching the given topic
func (idx *RuleIndex) Find(topic string) []*Rule {
    idx.mu.RLock()
    defer idx.mu.RUnlock()

    atomic.AddUint64(&idx.stats.lookups, 1)
    rules := idx.exactMatches[topic]
    if len(rules) > 0 {
        atomic.AddUint64(&idx.stats.matches, 1)
    }
    
    return rules
}

// GetTopics returns a list of all indexed topics
func (idx *RuleIndex) GetTopics() []string {
    idx.mu.RLock()
    defer idx.mu.RUnlock()
    
    topics := make([]string, 0, len(idx.exactMatches))
    for topic := range idx.exactMatches {
        topics = append(topics, topic)
    }
    return topics
}

// Clear removes all indexed rules
func (idx *RuleIndex) Clear() {
    idx.mu.Lock()
    defer idx.mu.Unlock()

    idx.exactMatches = make(map[string][]*Rule)
    idx.stats.lastUpdated = time.Now()
}

// GetStats returns current index statistics
func (idx *RuleIndex) GetStats() IndexStats {
    idx.stats.mu.RLock()
    defer idx.stats.mu.RUnlock()
    return idx.stats
}
