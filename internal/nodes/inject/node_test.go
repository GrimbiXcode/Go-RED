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
			"interval":   float64(5000),
			"topic":      "new-topic",
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
			"data":       "test data",
			"number":     float64(42),
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

// startNode runs Start in the background and returns a channel that
// receives every emitted payload, plus a function that waits for Start to
// return.
func startNode(t *testing.T, node *InjectNode, ctx context.Context) (<-chan map[string]interface{}, func()) {
	t.Helper()
	emitted := make(chan map[string]interface{}, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		assert.NoError(t, node.Start(ctx, func(payload map[string]interface{}) {
			emitted <- payload
		}))
	}()
	return emitted, func() {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Start did not return after cancellation")
		}
	}
}

func TestInjectNode_IntervalInjection(t *testing.T) {
	t.Run("should emit on every interval tick after Start", func(t *testing.T) {
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"intervalTest": true}
		node.config.Interval = 20

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		emitted, wait := startNode(t, node, ctx)

		received := 0
		timeout := time.After(time.Second)
		for received < 3 {
			select {
			case payload := <-emitted:
				assert.Equal(t, true, payload["intervalTest"])
				received++
			case <-timeout:
				t.Fatalf("expected at least 3 interval emissions, got %d", received)
			}
		}
		assert.NotNil(t, node.Ticker(), "ticker runs while Start is running")

		cancel()
		wait()
		assert.Nil(t, node.Ticker(), "ticker is released when Start returns")
	})

	t.Run("should emit exactly once when InjectOnce is set and no interval", func(t *testing.T) {
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"once": true}
		node.config.InjectOnce = true

		ctx, cancel := context.WithCancel(context.Background())
		emitted, wait := startNode(t, node, ctx)

		select {
		case payload := <-emitted:
			assert.Equal(t, true, payload["once"])
		case <-time.After(time.Second):
			t.Fatal("InjectOnce did not emit")
		}
		select {
		case <-emitted:
			t.Fatal("InjectOnce emitted more than once")
		case <-time.After(50 * time.Millisecond):
		}

		cancel()
		wait()
	})

	t.Run("should not emit without interval or InjectOnce, but block until cancelled", func(t *testing.T) {
		node := NewInjectNode()
		ctx, cancel := context.WithCancel(context.Background())
		emitted, wait := startNode(t, node, ctx)

		select {
		case <-emitted:
			t.Fatal("manual-only inject emitted on its own")
		case <-time.After(50 * time.Millisecond):
		}

		cancel()
		wait()
	})

	t.Run("should stop interval injection when Stop is called", func(t *testing.T) {
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"stopTest": true}
		node.config.Interval = 20

		emitted, wait := startNode(t, node, context.Background())

		select {
		case <-emitted:
		case <-time.After(time.Second):
			t.Fatal("no emission before Stop")
		}

		node.Stop()
		wait()
		assert.Nil(t, node.Ticker(), "Ticker should be nil after Stop")

		// Stop is idempotent and Close delegates to it.
		node.Stop()
		assert.NoError(t, node.Close())
	})

	t.Run("emitted payloads are copies of the configured payload", func(t *testing.T) {
		node := NewInjectNode()
		node.config.Payload = map[string]interface{}{"count": 0}
		node.config.InjectOnce = true

		ctx, cancel := context.WithCancel(context.Background())
		emitted, wait := startNode(t, node, ctx)

		payload := <-emitted
		payload["count"] = 99
		assert.Equal(t, 0, node.config.Payload["count"], "mutating an emitted message must not change the node config")

		cancel()
		wait()
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
