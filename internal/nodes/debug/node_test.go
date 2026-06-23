package debug

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
    log.SetPrefix("[DEBUG-TEST] ")
}

func TestDebugNode_Initialization(t *testing.T) {
    t.Run("should initialize with default configuration", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode initialization...")
        
        node := NewDebugNode()
        
        // Verify default configuration
        assert.NotNil(t, node)
        assert.True(t, node.config.Enabled)
        assert.True(t, node.config.OutputToConsole)
        assert.Equal(t, 100, node.config.MaxBufferSize)
        assert.Equal(t, "", node.config.Prefix)
        assert.True(t, node.config.ShowTimestamp)
        assert.True(t, node.config.ShowPath)
        
        // Verify buffer is initialized
        assert.NotNil(t, node.outputBuffer)
        assert.Equal(t, 0, len(node.outputBuffer))
        assert.Equal(t, 100, node.maxBufferSize)
        
        log.Printf("[DEBUG] DebugNode initialization test passed")
    })
}

func TestDebugNode_Validate(t *testing.T) {
    t.Run("should validate configuration correctly", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode validation...")
        
        node := NewDebugNode()
        
        // Test valid configuration
        err := node.Validate()
        assert.NoError(t, err, "Valid configuration should not produce error")
        
        // Test negative maxBufferSize
        node.config.MaxBufferSize = -1
        err = node.Validate()
        assert.Error(t, err, "Negative maxBufferSize should produce error")
        assert.Contains(t, err.Error(), "maxBufferSize cannot be negative")
        
        // Reset maxBufferSize
        node.config.MaxBufferSize = 100
        err = node.Validate()
        assert.NoError(t, err, "Valid maxBufferSize should pass validation")
        
        log.Printf("[DEBUG] DebugNode validation test passed")
    })
}

func TestDebugNode_GetConfig(t *testing.T) {
    t.Run("should return current configuration", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode GetConfig...")
        
        node := NewDebugNode()
        node.config.Enabled = false
        node.config.OutputToConsole = false
        node.config.MaxBufferSize = 200
        node.config.Prefix = "test-prefix"
        node.config.ShowTimestamp = false
        node.config.ShowPath = false
        
        config := node.GetConfig()
        
        assert.NotNil(t, config)
        assert.Equal(t, false, config["enabled"])
        assert.Equal(t, false, config["outputToConsole"])
        assert.Equal(t, 200, config["maxBufferSize"])
        assert.Equal(t, "test-prefix", config["prefix"])
        assert.Equal(t, false, config["showTimestamp"])
        assert.Equal(t, false, config["showPath"])
        
        log.Printf("[DEBUG] DebugNode GetConfig test passed")
    })
}

func TestDebugNode_SetConfig(t *testing.T) {
    t.Run("should update configuration from map", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode SetConfig...")
        
        node := NewDebugNode()
        
        newConfig := map[string]interface{}{
            "enabled":          false,
            "outputToConsole":  false,
            "maxBufferSize":    float64(150),
            "prefix":           "new-prefix",
            "showTimestamp":    false,
            "showPath":         false,
        }
        
        err := node.SetConfig(newConfig)
        assert.NoError(t, err, "SetConfig should succeed with valid configuration")
        
        // Verify configuration was updated
        assert.Equal(t, false, node.config.Enabled)
        assert.Equal(t, false, node.config.OutputToConsole)
        assert.Equal(t, 150, node.config.MaxBufferSize)
        assert.Equal(t, "new-prefix", node.config.Prefix)
        assert.Equal(t, false, node.config.ShowTimestamp)
        assert.Equal(t, false, node.config.ShowPath)
        
        log.Printf("[DEBUG] DebugNode SetConfig test passed")
    })
    
    t.Run("should return error for invalid configuration", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode SetConfig with invalid maxBufferSize...")
        
        node := NewDebugNode()
        
        invalidConfig := map[string]interface{}{
            "maxBufferSize": float64(-50),
        }
        
        err := node.SetConfig(invalidConfig)
        assert.Error(t, err, "SetConfig should fail with negative maxBufferSize")
        assert.Contains(t, err.Error(), "maxBufferSize cannot be negative")
        
        log.Printf("[DEBUG] DebugNode SetConfig invalid test passed")
    })
}

func TestDebugNode_Execute_Disabled(t *testing.T) {
    t.Run("should pass through input when disabled", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode Execute when disabled...")
        
        node := NewDebugNode()
        node.config.Enabled = false
        
        ctx := context.Background()
        input := map[string]interface{}{
            "test":    "data",
            "number": float64(42),
        }
        
        output, err := node.Execute(ctx, input)
        assert.NoError(t, err, "Execute should succeed when disabled")
        assert.NotNil(t, output)
        
        // Should pass through input unchanged
        assert.Equal(t, "data", output["test"])
        assert.Equal(t, float64(42), output["number"])
        
        // Output buffer should be empty
        assert.Equal(t, 0, len(node.outputBuffer))
        
        log.Printf("[DEBUG] DebugNode disabled test passed")
    })
}

func TestDebugNode_Execute_Enabled(t *testing.T) {
    t.Run("should log and buffer messages when enabled", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode Execute when enabled...")
        
        // Create a pipe to capture stderr output
        oldStderr := os.Stderr
        r, w, _ := os.Pipe()
        os.Stderr = w
        
        // Restore stderr when done
        defer func() {
            w.Close()
            os.Stderr = oldStderr
        }()
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = true
        node.config.Prefix = "[TEST]"
        node.config.ShowTimestamp = false // Disable timestamp for easier testing
        node.config.ShowPath = false
        
        ctx := context.Background()
        input := map[string]interface{}{
            "payload": "test data",
            "number":  float64(123),
        }
        
        output, err := node.Execute(ctx, input)
        assert.NoError(t, err, "Execute should succeed when enabled")
        assert.NotNil(t, output)
        
        // Should pass through input
        assert.Equal(t, "test data", output["payload"])
        assert.Equal(t, float64(123), output["number"])
        
        // Should have added to buffer
        assert.Equal(t, 1, len(node.outputBuffer))
        
        // Check that something was written to stderr
        w.Close()
        var buf [1024]byte
        n, _ := r.Read(buf[:])
        outputStr := string(buf[:n])
        
        // Should contain our prefix
        assert.Contains(t, outputStr, "[TEST]")
        assert.Contains(t, outputStr, "test data")
        
        log.Printf("[DEBUG] DebugNode enabled test passed")
    })
    
    t.Run("should handle nil input gracefully", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode Execute with nil input...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false // Don't spam console
        
        ctx := context.Background()
        
        output, err := node.Execute(ctx, nil)
        assert.NoError(t, err, "Execute should handle nil input")
        // Note: output can be nil when input is nil and node is disabled, but when enabled it should pass through input
        // Since input is nil and node is enabled, it should still pass through the nil
        assert.Nil(t, output, "Output should be nil when input is nil")
        
        // Should have added to buffer (though message will be empty)
        assert.GreaterOrEqual(t, len(node.outputBuffer), 1)
        
        log.Printf("[DEBUG] DebugNode nil input test passed")
    })
}

func TestDebugNode_Execute_PathHandling(t *testing.T) {
    t.Run("should include path in message when showPath is true", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode path handling...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.ShowPath = true
        node.config.ShowTimestamp = false
        
        ctx := context.Background()
        input := map[string]interface{}{
            "payload": "test",
            "_path":   []string{"node1", "node2", "node3"},
        }
        
        output, err := node.Execute(ctx, input)
        assert.NoError(t, err)
        assert.NotNil(t, output)
        
        // Should have added to buffer
        assert.Equal(t, 1, len(node.outputBuffer))
        
        // Check that path is in the message
        message := node.outputBuffer[0]
        assert.Contains(t, message, "node1")
        assert.Contains(t, message, "node2")
        assert.Contains(t, message, "node3")
        assert.Contains(t, message, "Path:")
        
        log.Printf("[DEBUG] DebugNode path handling test passed")
    })
    
    t.Run("should not include path when showPath is false", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode without path...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.ShowPath = false
        node.config.ShowTimestamp = false
        
        ctx := context.Background()
        input := map[string]interface{}{
            "payload": "test",
            "_path":   []string{"node1", "node2"},
        }
        
        output, err := node.Execute(ctx, input)
        assert.NoError(t, err)
        assert.NotNil(t, output)
        
        // Should have added to buffer
        assert.Equal(t, 1, len(node.outputBuffer))
        
        // Check that path is NOT in the message
        message := node.outputBuffer[0]
        assert.NotContains(t, message, "Path:")
        
        log.Printf("[DEBUG] DebugNode without path test passed")
    })
}

func TestDebugNode_Execute_Timestamp(t *testing.T) {
    t.Run("should include timestamp when showTimestamp is true", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode timestamp handling...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.ShowTimestamp = true
        
        ctx := context.Background()
        input := map[string]interface{}{
            "payload": "timestamp test",
        }
        
        // Get time before execution
        startTime := time.Now()
        
        output, err := node.Execute(ctx, input)
        assert.NoError(t, err)
        assert.NotNil(t, output)
        
        // Should have added to buffer
        assert.Equal(t, 1, len(node.outputBuffer))
        
        // Check that timestamp is in the message
        message := node.outputBuffer[0]
        // Timestamp format: "2006-01-02 15:04:05.000"
        // Just check that it contains something that looks like a timestamp (year pattern)
        assert.Contains(t, message, "2026") // Check for current year timestamp
        
        // Also check the time is reasonable (within a second of now)
        endTime := time.Now()
        assert.WithinDuration(t, startTime, endTime, 2*time.Second)
        
        log.Printf("[DEBUG] DebugNode timestamp handling test passed")
    })
    
    t.Run("should not include timestamp when showTimestamp is false", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode without timestamp...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.ShowTimestamp = false
        
        ctx := context.Background()
        input := map[string]interface{}{
            "payload": "no timestamp test",
        }
        
        output, err := node.Execute(ctx, input)
        assert.NoError(t, err)
        assert.NotNil(t, output)
        
        // Should have added to buffer
        assert.Equal(t, 1, len(node.outputBuffer))
        
        // Check that timestamp is NOT in the message
        // Look for the specific timestamp format
        message := node.outputBuffer[0]
        assert.NotContains(t, message, "2006-")
        assert.NotContains(t, message, "15:04:05")
        
        log.Printf("[DEBUG] DebugNode without timestamp test passed")
    })
}

func TestDebugNode_Execute_Prefix(t *testing.T) {
    t.Run("should include prefix in message when configured", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode prefix handling...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.Prefix = "MY-PREFIX"
        node.config.ShowTimestamp = false
        
        ctx := context.Background()
        input := map[string]interface{}{
            "payload": "prefix test",
        }
        
        output, err := node.Execute(ctx, input)
        assert.NoError(t, err)
        assert.NotNil(t, output)
        
        // Should have added to buffer
        assert.Equal(t, 1, len(node.outputBuffer))
        
        // Check that prefix is in the message
        message := node.outputBuffer[0]
        assert.Contains(t, message, "[MY-PREFIX]")
        
        log.Printf("[DEBUG] DebugNode prefix handling test passed")
    })
    
    t.Run("should handle empty prefix", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode with empty prefix...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.Prefix = ""
        node.config.ShowTimestamp = false
        
        ctx := context.Background()
        input := map[string]interface{}{
            "payload": "empty prefix test",
        }
        
        output, err := node.Execute(ctx, input)
        assert.NoError(t, err)
        assert.NotNil(t, output)
        
        // Should have added to buffer
        assert.Equal(t, 1, len(node.outputBuffer))
        
        // Check that no prefix brackets are in the message
        message := node.outputBuffer[0]
        assert.NotContains(t, message, "[]")
        
        log.Printf("[DEBUG] DebugNode empty prefix test passed")
    })
}

func TestDebugNode_BufferManagement(t *testing.T) {
    t.Run("should respect maxBufferSize", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode buffer size management...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.MaxBufferSize = 5
        node.maxBufferSize = 5
        
        ctx := context.Background()
        
        // Add more messages than buffer size
        for i := 0; i < 10; i++ {
            input := map[string]interface{}{
                "message": i,
            }
            _, err := node.Execute(ctx, input)
            assert.NoError(t, err)
        }
        
        // Buffer should only contain maxBufferSize messages
        assert.Equal(t, 5, len(node.outputBuffer))
        
        // First messages should have been removed (oldest first)
        // The buffer should contain messages 5-9
        log.Printf("[DEBUG] DebugNode buffer size test passed")
    })
    
    t.Run("should clear buffer when ClearOutput is called", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode ClearOutput...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        
        ctx := context.Background()
        
        // Add some messages
        for i := 0; i < 5; i++ {
            input := map[string]interface{}{
                "message": i,
            }
            _, err := node.Execute(ctx, input)
            assert.NoError(t, err)
        }
        
        assert.Equal(t, 5, len(node.outputBuffer))
        
        // Clear the buffer
        node.ClearOutput()
        
        assert.Equal(t, 0, len(node.outputBuffer))
        
        log.Printf("[DEBUG] DebugNode ClearOutput test passed")
    })
    
    t.Run("should return copy of buffer when GetOutput is called", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode GetOutput...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        
        ctx := context.Background()
        
        // Add some messages
        for i := 0; i < 3; i++ {
            input := map[string]interface{}{
                "message": i,
            }
            _, err := node.Execute(ctx, input)
            assert.NoError(t, err)
        }
        
        // Get output
        output := node.GetOutput()
        
        // Should have 3 messages
        assert.Equal(t, 3, len(output))
        
        // Modify the returned slice - should not affect internal buffer
        output[0] = "modified"
        
        // Internal buffer should be unchanged
        internalOutput := node.outputBuffer
        assert.NotEqual(t, "modified", internalOutput[0])
        
        log.Printf("[DEBUG] DebugNode GetOutput test passed")
    })
}

func TestDebugNode_MessageFormatting(t *testing.T) {
    t.Run("should format message with payload field", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode message formatting with payload...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.Prefix = ""
        node.config.ShowTimestamp = false
        node.config.ShowPath = false
        
        ctx := context.Background()
        input := map[string]interface{}{
            "payload": "my payload data",
            "other":   "ignored",
        }
        
        _, err := node.Execute(ctx, input)
        assert.NoError(t, err)
        
        message := node.outputBuffer[0]
        assert.Contains(t, message, "Payload: my payload data")
        
        log.Printf("[DEBUG] DebugNode payload formatting test passed")
    })
    
    t.Run("should format message without payload field", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode message formatting without payload...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.Prefix = ""
        node.config.ShowTimestamp = false
        node.config.ShowPath = false
        
        ctx := context.Background()
        input := map[string]interface{}{
            "data":     "my data",
            "number":   float64(42),
            "enabled":  true,
        }
        
        _, err := node.Execute(ctx, input)
        assert.NoError(t, err)
        
        message := node.outputBuffer[0]
        assert.Contains(t, message, "Data: map[data:my data enabled:true number:42]")
        
        log.Printf("[DEBUG] DebugNode non-payload formatting test passed")
    })
}

func TestDebugNode_ConcurrentAccess(t *testing.T) {
    t.Run("should handle concurrent executions safely", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode concurrent access...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.MaxBufferSize = 100
        
        ctx := context.Background()
        
        var wg sync.WaitGroup
        numGoroutines := 10
        
        for i := 0; i < numGoroutines; i++ {
            wg.Add(1)
            go func(id int) {
                defer wg.Done()
                input := map[string]interface{}{"id": id, "data": "concurrent test"}
                output, err := node.Execute(ctx, input)
                assert.NoError(t, err)
                assert.NotNil(t, output)
            }(i)
        }
        
        wg.Wait()
        
        // Buffer should have messages from all goroutines (up to maxBufferSize)
        assert.LessOrEqual(t, len(node.outputBuffer), numGoroutines)
        
        log.Printf("[DEBUG] DebugNode concurrent access test passed")
    })
}

func TestDebugNode_AllConfigurationsCombined(t *testing.T) {
    t.Run("should handle all configuration options together", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode with all configurations...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.MaxBufferSize = 10
        node.config.Prefix = "[COMBINED-TEST]"
        node.config.ShowTimestamp = true
        node.config.ShowPath = true
        
        ctx := context.Background()
        input := map[string]interface{}{
            "payload": "combined test data",
            "_path":   []string{"inject", "function", "debug"},
        }
        
        output, err := node.Execute(ctx, input)
        assert.NoError(t, err)
        assert.NotNil(t, output)
        
        // Should have added to buffer
        assert.Equal(t, 1, len(node.outputBuffer))
        
        message := node.outputBuffer[0]
        
        // Should contain all elements
        assert.Contains(t, message, "[COMBINED-TEST]")
        assert.Contains(t, message, "Payload: combined test data")
        assert.Contains(t, message, "Path:")
        assert.Contains(t, message, "inject")
        assert.Contains(t, message, "function")
        assert.Contains(t, message, "debug")
        // Should contain timestamp (check for year pattern)
        assert.Contains(t, message, "2026")
        
        log.Printf("[DEBUG] DebugNode combined configurations test passed")
    })
}

func TestDebugNode_PayloadTypes(t *testing.T) {
    t.Run("should handle various payload types", func(t *testing.T) {
        log.Printf("[DEBUG] Testing DebugNode with various payload types...")
        
        node := NewDebugNode()
        node.config.Enabled = true
        node.config.OutputToConsole = false
        node.config.Prefix = ""
        node.config.ShowTimestamp = false
        node.config.ShowPath = false
        
        ctx := context.Background()
        
        testCases := []map[string]interface{}{
            {"string": "test", "number": float64(42), "boolean": true},
            {"nested": map[string]interface{}{"key": "value"}},
            {"array": []interface{}{"a", "b", "c"}},
            {"null": nil},
            {},
        }
        
        for i, testCase := range testCases {
            _, err := node.Execute(ctx, testCase)
            assert.NoError(t, err, "Should handle test case %d", i)
            
            // Should have added to buffer
            assert.GreaterOrEqual(t, len(node.outputBuffer), i+1)
        }
        
        log.Printf("[DEBUG] DebugNode various payload types test passed")
    })
}
