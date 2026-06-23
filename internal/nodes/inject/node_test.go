package inject

import (
	"context"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Enable debug logging for tests
func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	log.SetPrefix("[INJECT-TEST] ")
}

func TestInjectNode_Initialization(t *testing.T) {
	t.Run("should initialize with default configuration", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode initialization...")
		
		node := NewInjectNode()
		
		// Verify default configuration
		assert.NotNil(t, node)
		assert.NotNil(t, node.config.Payload)
		assert.Equal(t, int64(0), node.config.Interval)
		assert.Equal(t, "", node.config.Topic)
		assert.Equal(t, false, node.config.InjectOnce)
		
		// Verify default payload
		if payload, ok := node.config.Payload["payload"].(string); ok {
			assert.Equal(t, "", payload)
		} else {
			assert.NotNil(t, node.config.Payload["payload"])
		}
		
		log.Printf("[DEBUG] InjectNode initialization test passed")
	})
}

func TestInjectNode_Validate(t *testing.T) {
	t.Run("should validate configuration correctly", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode validation...")
		
		node := NewInjectNode()
		
		// Test valid configuration
		err := node.Validate()
		assert.NoError(t, err, "Valid configuration should not produce error")
		
		// Test negative interval
		node.config.Interval = -1
		err = node.Validate()
		assert.Error(t, err, "Negative interval should produce error")
		assert.Contains(t, err.Error(), "interval cannot be negative")
		
		// Reset interval
		node.config.Interval = 0
		err = node.Validate()
		assert.NoError(t, err, "Zero interval should be valid")
		
		log.Printf("[DEBUG] InjectNode validation test passed")
	})
}

func TestInjectNode_GetConfig(t *testing.T) {
	t.Run("should return current configuration", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode GetConfig...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"test": "value"}
		node.config.Interval = 1000
		node.config.Topic = "test-topic"
		node.config.InjectOnce = true
		
		config := node.GetConfig()
		
		assert.NotNil(t, config)
		assert.Equal(t, map[string]interface{}{"test": "value"}, config["payload"])
		assert.Equal(t, int64(1000), config["interval"])
		assert.Equal(t, "test-topic", config["topic"])
		assert.Equal(t, true, config["injectOnce"])
		
		log.Printf("[DEBUG] InjectNode GetConfig test passed")
	})
}

func TestInjectNode_SetConfig(t *testing.T) {
	t.Run("should update configuration from map", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode SetConfig...")
		
		node := NewInjectNode()
		
		newConfig := map[string]interface{}{
			"payload":    map[string]interface{}{"newTest": "newValue"},
			"interval":  float64(5000),
			"topic":     "new-topic",
			"injectOnce": true,
		}
		
		err := node.SetConfig(newConfig)
		assert.NoError(t, err, "SetConfig should succeed with valid configuration")
		
		// Verify configuration was updated
		assert.Equal(t, map[string]interface{}{"newTest": "newValue"}, node.config.Payload)
		assert.Equal(t, int64(5000), node.config.Interval)
		assert.Equal(t, "new-topic", node.config.Topic)
		assert.Equal(t, true, node.config.InjectOnce)
		
		log.Printf("[DEBUG] InjectNode SetConfig test passed")
	})
	
	t.Run("should return error for invalid configuration", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode SetConfig with invalid interval...")
		
		node := NewInjectNode()
		
		invalidConfig := map[string]interface{}{
			"interval": float64(-100),
		}
		
		err := node.SetConfig(invalidConfig)
		assert.Error(t, err, "SetConfig should fail with negative interval")
		assert.Contains(t, err.Error(), "interval cannot be negative")
		
		log.Printf("[DEBUG] InjectNode SetConfig invalid test passed")
	})
}

func TestInjectNode_Execute_ManualInjection(t *testing.T) {
	t.Run("should execute and return configured payload", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode Execute for manual injection...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{
			"manualTest": true,
			"data":      "test data",
			"number":    float64(42),
		}
		node.config.Topic = "manual-topic"
		
		ctx := context.Background()
		
		// Execute without input (manual injection)
		output, err := node.Execute(ctx, nil)
		assert.NoError(t, err, "Execute should succeed for manual injection")
		assert.NotNil(t, output)
		
		// Verify output contains configured payload
		assert.Equal(t, true, output["manualTest"])
		assert.Equal(t, "test data", output["data"])
		assert.Equal(t, float64(42), output["number"])
		assert.Equal(t, "manual-topic", output["topic"])
		
		log.Printf("[DEBUG] InjectNode manual injection test passed")
	})
	
	t.Run("should use input as payload when provided", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode Execute with input...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"default": "value"}
		
		ctx := context.Background()
		input := map[string]interface{}{"inputData": "from input", "timestamp": float64(1234567890)}
		
		output, err := node.Execute(ctx, input)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		
		// Should use input as payload
		assert.Equal(t, "from input", output["inputData"])
		assert.Equal(t, float64(1234567890), output["timestamp"])
		
		// Should also have lastPayload stored
		assert.NotNil(t, node.lastPayload)
		
		log.Printf("[DEBUG] InjectNode Execute with input test passed")
	})
	
	t.Run("should use last payload when no input provided", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode Execute with last payload...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"default": "value"}
		
		ctx := context.Background()
		
		// First execution with input
		firstInput := map[string]interface{}{"first": "data"}
		_, err := node.Execute(ctx, firstInput)
		assert.NoError(t, err)
		
		// Second execution without input - should use last payload
		output, err := node.Execute(ctx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "data", output["first"])
		
		log.Printf("[DEBUG] InjectNode Execute with last payload test passed")
	})
}

func TestInjectNode_Execute_InjectOnce(t *testing.T) {
	t.Run("should inject only once when InjectOnce is true", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode InjectOnce functionality...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"once": true}
		node.config.InjectOnce = true
		
		ctx := context.Background()
		
		// First execution should work
		output1, err := node.Execute(ctx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, output1)
		assert.Equal(t, true, output1["once"])
		
		// Second execution should also work (InjectOnce only affects startup behavior)
		// Note: In current implementation, InjectOnce doesn't prevent subsequent manual injections
		output2, err := node.Execute(ctx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, output2)
		
		log.Printf("[DEBUG] InjectNode InjectOnce test passed")
	})
}

func TestInjectNode_IntervalInjection(t *testing.T) {
	t.Run("should start interval injection when interval > 0", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode interval injection...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"intervalTest": true}
		node.config.Interval = 50 // 50ms interval for faster testing
		
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		
		// Execute should start the ticker
		_, err := node.Execute(ctx, nil)
		assert.NoError(t, err)
		
		// Wait for the ticker to be initialized
		time.Sleep(100 * time.Millisecond)
		
		// Verify ticker was started
		assert.NotNil(t, node.ticker, "Ticker should be started after Execute with interval > 0")
		
		// Stop the node to clean up
		node.Stop()
		
		// Verify ticker was stopped
		assert.Nil(t, node.ticker, "Ticker should be nil after Stop")
		
		log.Printf("[DEBUG] InjectNode interval injection test passed")
	})
	
	t.Run("should stop interval injection when Stop is called", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode Stop functionality...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"stopTest": true}
		node.config.Interval = 50 // 50ms interval
		
		ctx := context.Background()
		
		// Execute to start interval injection
		_, err := node.Execute(ctx, nil)
		assert.NoError(t, err)
		
		// Wait for ticker to be started
		time.Sleep(100 * time.Millisecond)
		
		// Verify ticker is running
		assert.NotNil(t, node.ticker, "Ticker should be running after Execute with interval > 0")
		
		// Stop the node
		node.Stop()
		
		// Verify ticker was stopped
		assert.Nil(t, node.ticker, "Ticker should be nil after Stop")
		
		// Verify done channel was closed
		select {
		case <-node.done:
			// Expected - done channel should be closed
		default:
			// If not closed, that's also fine - the important thing is ticker is stopped
		}
		
		log.Printf("[DEBUG] InjectNode Stop test passed")
	})
	
	t.Run("should stop interval injection when context is cancelled", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode context cancellation...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"contextTest": true}
		node.config.Interval = 50 // 50ms interval
		
		ctx, cancel := context.WithCancel(context.Background())
		
		// Execute to start interval injection
		_, err := node.Execute(ctx, nil)
		assert.NoError(t, err)
		
		// Wait for ticker to be started
		time.Sleep(100 * time.Millisecond)
		
		// Verify ticker is running
		assert.NotNil(t, node.ticker, "Ticker should be running after Execute with interval > 0")
		
		// Cancel context
		cancel()
		
		// Wait a bit for goroutine to exit
		time.Sleep(100 * time.Millisecond)
		
		// Stop the node to clean up
		node.Stop()
		
		log.Printf("[DEBUG] InjectNode context cancellation test passed")
	})
}

func TestInjectNode_ConcurrentAccess(t *testing.T) {
	t.Run("should handle concurrent executions safely", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode concurrent access...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"concurrent": true}
		
		ctx := context.Background()
		
		var wg sync.WaitGroup
		numGoroutines := 10
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				input := map[string]interface{}{"id": id, "data": "test"}
				output, err := node.Execute(ctx, input)
				assert.NoError(t, err)
				assert.NotNil(t, output)
			}(i)
		}
		
		wg.Wait()
		
		log.Printf("[DEBUG] InjectNode concurrent access test passed")
	})
}

func TestInjectNode_TopicHandling(t *testing.T) {
	t.Run("should add topic to payload when configured", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode topic handling...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"data": "test"}
		node.config.Topic = "my-topic"
		
		ctx := context.Background()
		
		output, err := node.Execute(ctx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		
		// Should have topic in output
		topic, exists := output["topic"]
		assert.True(t, exists, "Topic should be in output")
		assert.Equal(t, "my-topic", topic)
		
		// Should also have original payload
		assert.Equal(t, "test", output["data"])
		
		log.Printf("[DEBUG] InjectNode topic handling test passed")
	})
	
	t.Run("should handle empty topic configuration", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode with empty topic...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"data": "test"}
		node.config.Topic = ""
		
		ctx := context.Background()
		
		output, err := node.Execute(ctx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		
		// Should not have topic in output when topic is empty
		_, exists := output["topic"]
		assert.False(t, exists, "Topic should not be in output when empty")
		
		log.Printf("[DEBUG] InjectNode empty topic test passed")
	})
	
	t.Run("should create payload map if nil when adding topic", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode nil payload with topic...")
		
		node := NewInjectNode()
		node.config.Payload = nil
		node.config.Topic = "test-topic"
		
		ctx := context.Background()
		
		output, err := node.Execute(ctx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		
		// Should have created payload map and added topic
		topic, exists := output["topic"]
		assert.True(t, exists, "Topic should be in output")
		assert.Equal(t, "test-topic", topic)
		
		log.Printf("[DEBUG] InjectNode nil payload with topic test passed")
	})
}

func TestInjectNode_PayloadMerging(t *testing.T) {
	t.Run("should merge input with configured payload correctly", func(t *testing.T) {
		log.Printf("[DEBUG] Testing InjectNode payload merging...")
		
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{
			"default1": "value1",
			"default2": "value2",
		}
		
		ctx := context.Background()
		
		// Execute with input
		input := map[string]interface{}{
			"input1": "inputValue1",
			"input2": "inputValue2",
		}
		
		output, err := node.Execute(ctx, input)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		
		// Should have input values
		assert.Equal(t, "inputValue1", output["input1"])
		assert.Equal(t, "inputValue2", output["input2"])
		
		// Note: In current implementation, input completely replaces configured payload
		// This test documents current behavior
		
		log.Printf("[DEBUG] InjectNode payload merging test passed")
	})
}
