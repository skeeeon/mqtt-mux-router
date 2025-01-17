package rule

import (
    "sync"
)

// ProcessingMessage represents a message being processed
type ProcessingMessage struct {
    Topic   string
    Payload []byte
    Values  map[string]interface{}
    Rules   []*Rule
    Actions []*Action
}

// MessagePool manages message object reuse
type MessagePool struct {
    pool sync.Pool
}

// NewMessagePool creates a new message pool
func NewMessagePool() *MessagePool {
    return &MessagePool{
        pool: sync.Pool{
            New: func() interface{} {
                return &ProcessingMessage{
                    Values:  make(map[string]interface{}, 10),
                    Rules:   make([]*Rule, 0, 5),
                    Actions: make([]*Action, 0, 5),
                }
            },
        },
    }
}

// Get retrieves a message object from the pool
func (p *MessagePool) Get() *ProcessingMessage {
    msg := p.pool.Get().(*ProcessingMessage)
    return msg
}

// Put returns a message object to the pool
func (p *MessagePool) Put(msg *ProcessingMessage) {
    // Clear message data before returning to pool
    msg.Topic = ""
    msg.Payload = msg.Payload[:0]
    for k := range msg.Values {
        delete(msg.Values, k)
    }
    msg.Rules = msg.Rules[:0]
    msg.Actions = msg.Actions[:0]
    p.pool.Put(msg)
}

// ProcessingResult represents the outcome of message processing
type ProcessingResult struct {
    Message *ProcessingMessage
    Error   error
}

// ResultPool manages result object reuse
type ResultPool struct {
    pool sync.Pool
}

// NewResultPool creates a new result pool
func NewResultPool() *ResultPool {
    return &ResultPool{
        pool: sync.Pool{
            New: func() interface{} {
                return &ProcessingResult{}
            },
        },
    }
}

// Get retrieves a result object from the pool
func (p *ResultPool) Get() *ProcessingResult {
    return p.pool.Get().(*ProcessingResult)
}

// Put returns a result object to the pool
func (p *ResultPool) Put(result *ProcessingResult) {
    result.Message = nil
    result.Error = nil
    p.pool.Put(result)
}
