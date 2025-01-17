package stats

import (
    "encoding/json"
    "sync/atomic"
    "time"
)

// StatsCollector manages application-wide statistics
type StatsCollector struct {
    StartTime         time.Time
    MessagesReceived  uint64
    MessagesProcessed uint64
    RulesMatched      uint64
    ActionsExecuted   uint64
    Errors           uint64
    LastUpdate       time.Time
}

// NewStatsCollector creates a new stats collector
func NewStatsCollector() *StatsCollector {
    return &StatsCollector{
        StartTime:  time.Now(),
        LastUpdate: time.Now(),
    }
}

// Update updates the stats with new values
func (s *StatsCollector) Update(received, processed, matched, executed, errors uint64) {
    atomic.StoreUint64(&s.MessagesReceived, received)
    atomic.StoreUint64(&s.MessagesProcessed, processed)
    atomic.StoreUint64(&s.RulesMatched, matched)
    atomic.StoreUint64(&s.ActionsExecuted, executed)
    atomic.StoreUint64(&s.Errors, errors)
    s.LastUpdate = time.Now()
}

// GetStats returns current statistics
func (s *StatsCollector) GetStats() map[string]interface{} {
    uptime := time.Since(s.StartTime)
    return map[string]interface{}{
        "uptime":            uptime.String(),
        "messages_received": atomic.LoadUint64(&s.MessagesReceived),
        "messages_processed": atomic.LoadUint64(&s.MessagesProcessed),
        "rules_matched":     atomic.LoadUint64(&s.RulesMatched),
        "actions_executed":  atomic.LoadUint64(&s.ActionsExecuted),
        "errors":           atomic.LoadUint64(&s.Errors),
        "last_update":      s.LastUpdate,
    }
}

// GetStatsJSON returns stats as JSON
func (s *StatsCollector) GetStatsJSON() ([]byte, error) {
    return json.Marshal(s.GetStats())
}

// CalculateRate calculates message processing rate
func (s *StatsCollector) CalculateRate() float64 {
    uptime := time.Since(s.StartTime).Seconds()
    if uptime <= 0 {
        return 0
    }
    return float64(s.MessagesProcessed) / uptime
}
