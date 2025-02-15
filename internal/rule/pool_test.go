package rule

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"mqtt-mux-router/internal/logger"
)

// Test setup helpers
func setupTestPool(t *testing.T) (*MessagePool, *ResultPool) {
	zapLogger, err := zap.NewDevelopment()
	assert.NoError(t, err)
	log := &logger.Logger{Logger: zapLogger}
	
	return NewMessagePool(log), NewResultPool(log)
}

func TestNewMessagePool(t *testing.T) {
	zapLogger, err := zap.NewDevelopment()
	assert.NoError(t, err)
	log := &logger.Logger{Logger: zapLogger}

	tests := []struct {
		name    string
		log     *logger.Logger
		wantNil bool
	}{
		{
			name:    "valid initialization",
			log:     log,
			wantNil: false,
		},
		{
			name:    "nil logger still works",
			log:     nil,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewMessagePool(tt.log)
			if tt.wantNil {
				assert.Nil(t, pool)
			} else {
				assert.NotNil(t, pool)
				assert.NotNil(t, pool.pool)
				// Initial stats should be zero
				stats := getPoolStats(&pool.stats)
				assert.Equal(t, uint64(0), stats["gets"])
				assert.Equal(t, uint64(0), stats["puts"])
				assert.Equal(t, uint64(0), stats["misses"])
			}
		})
	}
}

func TestMessagePool_GetPut(t *testing.T) {
	pool, _ := setupTestPool(t)

	t.Run("basic get and put", func(t *testing.T) {
		// Get a new message
		msg := pool.Get()
		assert.NotNil(t, msg)
		assert.NotNil(t, msg.Values)
		assert.NotNil(t, msg.Rules)
		assert.NotNil(t, msg.Actions)

		// Add some data
		msg.Topic = "test/topic"
		msg.Payload = []byte("test payload")
		msg.Values["key"] = "value"
		msg.Rules = append(msg.Rules, &Rule{Topic: "test"})
		msg.Actions = append(msg.Actions, &Action{Topic: "test"})

		// Put it back
		pool.Put(msg)

		// Get another message
		msg2 := pool.Get()
		assert.NotNil(t, msg2)
		// Verify it's clean
		assert.Empty(t, msg2.Topic)
		assert.Empty(t, msg2.Payload)
		assert.Empty(t, msg2.Values)
		assert.Empty(t, msg2.Rules)
		assert.Empty(t, msg2.Actions)
	})

	t.Run("nil message handling", func(t *testing.T) {
		pool.Put(nil) // Should not panic
		stats := getPoolStats(&pool.stats)
		assert.Greater(t, stats["gets"], uint64(0))
		assert.Greater(t, stats["puts"], uint64(0))
	})

	t.Run("concurrent usage", func(t *testing.T) {
		const goroutines = 10
		const iterations = 100
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					msg := pool.Get()
					assert.NotNil(t, msg)
					// Simulate some work
					time.Sleep(time.Millisecond)
					pool.Put(msg)
				}
			}()
		}

		wg.Wait()
		stats := getPoolStats(&pool.stats)
		assert.GreaterOrEqual(t, stats["gets"], uint64(goroutines*iterations))
		assert.GreaterOrEqual(t, stats["puts"], uint64(goroutines*iterations))
	})
}

func TestNewResultPool(t *testing.T) {
	zapLogger, err := zap.NewDevelopment()
	assert.NoError(t, err)
	log := &logger.Logger{Logger: zapLogger}

	tests := []struct {
		name    string
		log     *logger.Logger
		wantNil bool
	}{
		{
			name:    "valid initialization",
			log:     log,
			wantNil: false,
		},
		{
			name:    "nil logger still works",
			log:     nil,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewResultPool(tt.log)
			if tt.wantNil {
				assert.Nil(t, pool)
			} else {
				assert.NotNil(t, pool)
				assert.NotNil(t, pool.pool)
				// Initial stats should be zero
				stats := getPoolStats(&pool.stats)
				assert.Equal(t, uint64(0), stats["gets"])
				assert.Equal(t, uint64(0), stats["puts"])
				assert.Equal(t, uint64(0), stats["misses"])
			}
		})
	}
}

func TestResultPool_GetPut(t *testing.T) {
	_, pool := setupTestPool(t)

	t.Run("basic get and put", func(t *testing.T) {
		// Get a new result
		result := pool.Get()
		assert.NotNil(t, result)
		
		// Add some data
		result.Message = &ProcessingMessage{
			Topic: "test/topic",
			Payload: []byte("test payload"),
		}
		result.Error = assert.AnError

		// Put it back
		pool.Put(result)

		// Get another result
		result2 := pool.Get()
		assert.NotNil(t, result2)
		// Verify it's clean
		assert.Nil(t, result2.Message)
		assert.Nil(t, result2.Error)
	})

	t.Run("nil result handling", func(t *testing.T) {
		pool.Put(nil) // Should not panic
		stats := getPoolStats(&pool.stats)
		assert.Greater(t, stats["gets"], uint64(0))
		assert.Greater(t, stats["puts"], uint64(0))
	})

	t.Run("concurrent usage", func(t *testing.T) {
		const goroutines = 10
		const iterations = 100
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					result := pool.Get()
					assert.NotNil(t, result)
					// Simulate some work
					time.Sleep(time.Millisecond)
					pool.Put(result)
				}
			}()
		}

		wg.Wait()
		stats := getPoolStats(&pool.stats)
		assert.GreaterOrEqual(t, stats["gets"], uint64(goroutines*iterations))
		assert.GreaterOrEqual(t, stats["puts"], uint64(goroutines*iterations))
	})
}

func TestGetPoolStats(t *testing.T) {
	msgPool, resultPool := setupTestPool(t)

	// Use both pools a few times
	for i := 0; i < 3; i++ {
		msg := msgPool.Get()
		msgPool.Put(msg)

		result := resultPool.Get()
		resultPool.Put(result)
	}

	t.Run("message pool stats", func(t *testing.T) {
		stats := getPoolStats(&msgPool.stats)
		assert.Equal(t, uint64(3), stats["gets"])
		assert.Equal(t, uint64(3), stats["puts"])
		assert.Equal(t, uint64(1), stats["misses"]) // First Get causes a miss
	})

	t.Run("result pool stats", func(t *testing.T) {
		stats := getPoolStats(&resultPool.stats)
		assert.Equal(t, uint64(3), stats["gets"])
		assert.Equal(t, uint64(3), stats["puts"])
		assert.Equal(t, uint64(1), stats["misses"]) // First Get causes a miss
	})
}
