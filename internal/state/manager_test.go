package state

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/engine"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNewFileStateManager(t *testing.T) {
    t.Run("should create file state manager with default directory", func(t *testing.T) {
        // Create a temporary directory
        tmpDir, err := os.MkdirTemp("", "go-red-test")
        require.NoError(t, err)
        defer os.RemoveAll(tmpDir)

        manager, err := NewFileStateManager(tmpDir)
        require.NoError(t, err)
        assert.NotNil(t, manager)
    })

    t.Run("should fail to create state manager with non-existent directory", func(t *testing.T) {
        _, err := NewFileStateManager("/non/existent/directory")
        assert.Error(t, err)
    })
}

func TestFileStateManagerSaveAndLoadFlow(t *testing.T) {
    // Create a temporary directory
    tmpDir, err := os.MkdirTemp("", "go-red-test")
    require.NoError(t, err)
    defer os.RemoveAll(tmpDir)

    manager, err := NewFileStateManager(tmpDir)
    require.NoError(t, err)

    t.Run("should save and load flow", func(t *testing.T) {
        // Create a flow
        flow := engine.NewFlow("test-flow", "Test Flow")
        flow.Description = "A test flow description"
        flow.AddNode(&engine.Node{
            ID:   "node-1",
            Type: "function",
            X:    100,
            Y:    200,
        })
        flow.AddNode(&engine.Node{
            ID:   "node-2",
            Type: "debug",
            X:    300,
            Y:    400,
        })

        // Add a connection
        conn := engine.NodeConnection{
            ID:          "conn-1",
            SourceNode:  "node-1",
            SourcePort:  "output",
            TargetNode:  "node-2",
            TargetPort:  "input",
        }
        flow.AddConnection(conn)

        // Save the flow
        err = manager.SaveFlow(flow)
        require.NoError(t, err)

        // Load the flow
        loadedFlow, err := manager.LoadFlow("test-flow")
        require.NoError(t, err)
        assert.NotNil(t, loadedFlow)

        // Verify the loaded flow matches the saved flow
        assert.Equal(t, flow.ID, loadedFlow.ID)
        assert.Equal(t, flow.Name, loadedFlow.Name)
        assert.Equal(t, flow.Description, loadedFlow.Description)

        // Check nodes
        assert.Len(t, loadedFlow.Nodes, 2)
        assert.Contains(t, loadedFlow.Nodes, "node-1")
        assert.Contains(t, loadedFlow.Nodes, "node-2")
        assert.Equal(t, float64(100), loadedFlow.Nodes["node-1"].X)
        assert.Equal(t, float64(200), loadedFlow.Nodes["node-1"].Y)

        // Check connections
        assert.Len(t, loadedFlow.Connections, 1)
        assert.Equal(t, "conn-1", loadedFlow.Connections[0].ID)
        assert.Equal(t, "node-1", loadedFlow.Connections[0].SourceNode)
        assert.Equal(t, "node-2", loadedFlow.Connections[0].TargetNode)
    })

    t.Run("should fail to load non-existent flow", func(t *testing.T) {
        _, err := manager.LoadFlow("non-existent")
        assert.Error(t, err)
    })

    t.Run("should update existing flow", func(t *testing.T) {
        // Create and save initial flow
        flow := engine.NewFlow("update-test", "Initial Name")
        flow.AddNode(&engine.Node{ID: "node-1", Type: "function", X: 100, Y: 200})

        err := manager.SaveFlow(flow)
        require.NoError(t, err)

        // Modify the flow
        flow.Name = "Updated Name"
        flow.Description = "Updated description"
        flow.AddNode(&engine.Node{ID: "node-2", Type: "debug", X: 300, Y: 400})

        // Save again
        err = manager.SaveFlow(flow)
        require.NoError(t, err)

        // Load and verify
        loadedFlow, err := manager.LoadFlow("update-test")
        require.NoError(t, err)
        assert.Equal(t, "Updated Name", loadedFlow.Name)
        assert.Equal(t, "Updated description", loadedFlow.Description)
        assert.Len(t, loadedFlow.Nodes, 2)
    })
}

func TestFileStateManagerDeleteFlow(t *testing.T) {
    // Create a temporary directory
    tmpDir, err := os.MkdirTemp("", "go-red-test")
    require.NoError(t, err)
    defer os.RemoveAll(tmpDir)

    manager, err := NewFileStateManager(tmpDir)
    require.NoError(t, err)

    t.Run("should delete flow", func(t *testing.T) {
        // Create and save a flow
        flow := engine.NewFlow("delete-test", "Delete Test Flow")
        flow.AddNode(&engine.Node{ID: "node-1", Type: "function", X: 100, Y: 200})

        err := manager.SaveFlow(flow)
        require.NoError(t, err)

        // Verify it exists
        _, err = manager.LoadFlow("delete-test")
        require.NoError(t, err)

        // Delete the flow
        err = manager.DeleteFlow("delete-test")
        require.NoError(t, err)

        // Verify it's deleted
        _, err = manager.LoadFlow("delete-test")
        assert.Error(t, err)
    })

    t.Run("should fail to delete non-existent flow", func(t *testing.T) {
        err := manager.DeleteFlow("non-existent")
        assert.Error(t, err)
    })
}

func TestFileStateManagerLoadAllFlows(t *testing.T) {
    // Create a temporary directory
    tmpDir, err := os.MkdirTemp("", "go-red-test")
    require.NoError(t, err)
    defer os.RemoveAll(tmpDir)

    manager, err := NewFileStateManager(tmpDir)
    require.NoError(t, err)

    t.Run("should load all flows", func(t *testing.T) {
        // Create and save multiple flows
        for i := 1; i <= 5; i++ {
            flow := engine.NewFlow("list-test-"+string(rune('a'+i-1)), "Flow "+string(rune('A'+i-1)))
            flow.AddNode(&engine.Node{ID: "node-1", Type: "function", X: 100, Y: 200})
            err := manager.SaveFlow(flow)
            require.NoError(t, err)
        }

        // Load all flows
        flows, err := manager.LoadAllFlows()
        require.NoError(t, err)
        assert.Len(t, flows, 5)

        // Verify flow IDs
        flowIDs := make([]string, len(flows))
        for i, flow := range flows {
            flowIDs[i] = flow.ID
        }
        for i := 1; i <= 5; i++ {
            assert.Contains(t, flowIDs, "list-test-"+string(rune('a'+i-1)))
        }
    })

    t.Run("should return empty for no flows", func(t *testing.T) {
        // Use a fresh manager
        emptyDir, err := os.MkdirTemp("", "go-red-empty-test")
        require.NoError(t, err)
        defer os.RemoveAll(emptyDir)

        emptyManager, err := NewFileStateManager(emptyDir)
        require.NoError(t, err)

        flows, err := emptyManager.LoadAllFlows()
        require.NoError(t, err)
        assert.Len(t, flows, 0)
    })
}

func TestFileStateManagerDirectoryStructure(t *testing.T) {
    // Create a temporary directory
    tmpDir, err := os.MkdirTemp("", "go-red-test")
    require.NoError(t, err)
    defer os.RemoveAll(tmpDir)

    manager, err := NewFileStateManager(tmpDir)
    require.NoError(t, err)

    t.Run("should create directory structure on save", func(t *testing.T) {
        flow := engine.NewFlow("dir-test", "Directory Test Flow")
        flow.AddNode(&engine.Node{ID: "node-1", Type: "function", X: 100, Y: 200})

        err := manager.SaveFlow(flow)
        require.NoError(t, err)

        // Verify the directory structure was created
        flowsDir := filepath.Join(tmpDir, "flows")
        _, err = os.Stat(flowsDir)
        assert.NoError(t, err)

        // Verify the flow file exists
        flowPath := filepath.Join(flowsDir, "dir-test.json")
        _, err = os.Stat(flowPath)
        assert.NoError(t, err)
    })

    t.Run("should create base path if not exists", func(t *testing.T) {
        // Use a new temporary directory
        newDir := filepath.Join(tmpDir, "new", "nested", "path")
        manager, err := NewFileStateManager(newDir)
        require.NoError(t, err)
        assert.NotNil(t, manager)

        // Verify the directory was created
        _, err = os.Stat(newDir)
        assert.NoError(t, err)

        // Verify flows subdirectory was created
        flowsDir := filepath.Join(newDir, "flows")
        _, err = os.Stat(flowsDir)
        assert.NoError(t, err)
    })
}

func TestFileStateManagerConcurrentOperations(t *testing.T) {
    t.Run("should handle concurrent flow operations", func(t *testing.T) {
        tmpDir, err := os.MkdirTemp("", "go-red-concurrent-test")
        require.NoError(t, err)
        defer os.RemoveAll(tmpDir)

        manager, err := NewFileStateManager(tmpDir)
        require.NoError(t, err)

        // Create flows concurrently
        done := make(chan error, 10)
        for i := 0; i < 10; i++ {
            go func(id int) {
                flow := engine.NewFlow("concurrent-"+string(rune('a'+id)), "Concurrent Flow "+string(rune('A'+id)))
                flow.AddNode(&engine.Node{ID: "node-1", Type: "function", X: 100, Y: 200})
                done <- manager.SaveFlow(flow)
            }(i)
        }

        // Wait for all operations to complete
        for i := 0; i < 10; i++ {
            err := <-done
            assert.NoError(t, err)
        }

        // Verify all flows were saved
        flows, err := manager.LoadAllFlows()
        require.NoError(t, err)
        assert.Len(t, flows, 10)
    })
}

func TestFileStateManagerFlowPath(t *testing.T) {
    t.Run("should use correct flow file path", func(t *testing.T) {
        tmpDir, err := os.MkdirTemp("", "go-red-path-test")
        require.NoError(t, err)
        defer os.RemoveAll(tmpDir)

        manager, err := NewFileStateManager(tmpDir)
        require.NoError(t, err)

        // Save a flow
        flow := engine.NewFlow("path-test", "Path Test Flow")
        flow.AddNode(&engine.Node{ID: "node-1", Type: "function", X: 100, Y: 200})

        err = manager.SaveFlow(flow)
        require.NoError(t, err)

        // Verify the file exists at the expected path
        expectedPath := filepath.Join(tmpDir, "flows", "path-test.json")
        _, err = os.Stat(expectedPath)
        assert.NoError(t, err)
    })
}
