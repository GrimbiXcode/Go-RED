package registry

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNewNodeRegistry(t *testing.T) {
    t.Run("should create new node registry", func(t *testing.T) {
        registry := NewNodeRegistry()
        assert.NotNil(t, registry)
        assert.NotNil(t, registry.nodes)
    })
}

func TestNodeRegistryRegisterNode(t *testing.T) {
    t.Run("should register node type", func(t *testing.T) {
        registry := NewNodeRegistry()

        node := &Node{
            Type: "test-node",
            Metadata: NodeMetadata{
                ID:          "test-node",
                Name:        "Test Node",
                Description: "A test node type",
                Category:    "test",
                Icon:        "test-icon",
                Inputs:      []Port{{ID: "input1", Name: "Input 1"}},
                Outputs:     []Port{{ID: "output1", Name: "Output 1"}},
                ConfigSchema: Schema{
                    Properties: map[string]Property{
                        "property1": {Type: "string", Default: "default"},
                    },
                },
            },
            Factory: func() NodeExecutor {
                return &MockNodeExecutor{}
            },
        }

        err := registry.RegisterNode(node)
        require.NoError(t, err)

        // Verify the node was registered
        metadata, err := registry.GetMetadata("test-node")
        require.NoError(t, err)
        assert.Equal(t, node.Type, metadata.ID)
        assert.Equal(t, node.Metadata.Name, metadata.Name)
    })

    t.Run("should fail to register node with duplicate ID", func(t *testing.T) {
        registry := NewNodeRegistry()

        node1 := &Node{
            Type: "duplicate-node",
            Metadata: NodeMetadata{
                ID:   "duplicate-node",
                Name: "First Node",
            },
            Factory: func() NodeExecutor {
                return &MockNodeExecutor{}
            },
        }
        node2 := &Node{
            Type: "duplicate-node",
            Metadata: NodeMetadata{
                ID:   "duplicate-node",
                Name: "Second Node",
            },
            Factory: func() NodeExecutor {
                return &MockNodeExecutor{}
            },
        }

        err := registry.RegisterNode(node1)
        require.NoError(t, err)

        err = registry.RegisterNode(node2)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "already registered")
    })

    t.Run("should fail to register node with empty ID", func(t *testing.T) {
        registry := NewNodeRegistry()

        node := &Node{
            Type: "",
            Metadata: NodeMetadata{
                ID:   "",
                Name: "Empty ID Node",
            },
            Factory: func() NodeExecutor {
                return &MockNodeExecutor{}
            },
        }

        err := registry.RegisterNode(node)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "cannot be empty")
    })
}

func TestNodeRegistryGetNode(t *testing.T) {
    t.Run("should get registered node metadata", func(t *testing.T) {
        registry := NewNodeRegistry()

        node := &Node{
            Type: "get-test-node",
            Metadata: NodeMetadata{
                ID:   "get-test-node",
                Name: "Get Test Node",
            },
            Factory: func() NodeExecutor {
                return &MockNodeExecutor{}
            },
        }

        registry.RegisterNode(node)

        metadata, err := registry.GetMetadata("get-test-node")
        require.NoError(t, err)
        assert.Equal(t, node.Metadata.ID, metadata.ID)
        assert.Equal(t, node.Metadata.Name, metadata.Name)
    })

    t.Run("should fail to get non-existent node type", func(t *testing.T) {
        registry := NewNodeRegistry()

        _, err := registry.GetMetadata("non-existent")
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "not found")
    })
}

func TestNodeRegistryGetAllNodes(t *testing.T) {
    t.Run("should return all registered node types", func(t *testing.T) {
        registry := NewNodeRegistry()

        // Register multiple node types
        nodes := []*Node{
            {Type: "node-1", Metadata: NodeMetadata{ID: "node-1", Name: "Node 1"}, Factory: func() NodeExecutor { return &MockNodeExecutor{} }},
            {Type: "node-2", Metadata: NodeMetadata{ID: "node-2", Name: "Node 2"}, Factory: func() NodeExecutor { return &MockNodeExecutor{} }},
            {Type: "node-3", Metadata: NodeMetadata{ID: "node-3", Name: "Node 3"}, Factory: func() NodeExecutor { return &MockNodeExecutor{} }},
        }

        for _, node := range nodes {
            registry.RegisterNode(node)
        }

        allNodes := registry.GetAllNodes()
        assert.Len(t, allNodes, 3)

        // Verify all node types are present
        ids := make([]string, len(allNodes))
        for i, node := range allNodes {
            ids[i] = node.ID
        }
        assert.Contains(t, ids, "node-1")
        assert.Contains(t, ids, "node-2")
        assert.Contains(t, ids, "node-3")
    })

    t.Run("should return empty for no registered nodes", func(t *testing.T) {
        registry := NewNodeRegistry()

        allNodes := registry.GetAllNodes()
        assert.Len(t, allNodes, 0)
    })
}

func TestNodeRegistryUnregisterNode(t *testing.T) {
    t.Run("should unregister node type", func(t *testing.T) {
        registry := NewNodeRegistry()

        node := &Node{
            Type: "unregister-test",
            Metadata: NodeMetadata{
                ID:   "unregister-test",
                Name: "Unregister Test",
            },
            Factory: func() NodeExecutor {
                return &MockNodeExecutor{}
            },
        }

        registry.RegisterNode(node)

        // Verify it's registered
        _, err := registry.GetMetadata("unregister-test")
        require.NoError(t, err)

        // Unregister
        err = registry.Unregister("unregister-test")
        require.NoError(t, err)

        // Verify it's unregistered
        _, err = registry.GetMetadata("unregister-test")
        assert.Error(t, err)
    })

    t.Run("should fail to unregister non-existent node type", func(t *testing.T) {
        registry := NewNodeRegistry()

        err := registry.Unregister("non-existent")
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "not found")
    })
}

func TestNodeRegistryHasNode(t *testing.T) {
    t.Run("should return true for registered node type", func(t *testing.T) {
        registry := NewNodeRegistry()

        node := &Node{
            Type: "has-test",
            Metadata: NodeMetadata{
                ID:   "has-test",
                Name: "Has Test",
            },
            Factory: func() NodeExecutor {
                return &MockNodeExecutor{}
            },
        }

        registry.RegisterNode(node)

        assert.True(t, registry.IsRegistered("has-test"))
    })

    t.Run("should return false for non-existent node type", func(t *testing.T) {
        registry := NewNodeRegistry()

        assert.False(t, registry.IsRegistered("non-existent"))
    })
}

func TestNodeRegistryCreateNode(t *testing.T) {
    t.Run("should create node using registered node type", func(t *testing.T) {
        registry := NewNodeRegistry()

        createNodeCalled := false
        mockExecutor := &MockNodeExecutor{}

        node := &Node{
            Type: "create-test",
            Metadata: NodeMetadata{
                ID:   "create-test",
                Name: "Create Test",
            },
            Factory: func() NodeExecutor {
                createNodeCalled = true
                return mockExecutor
            },
        }

        registry.RegisterNode(node)

        executor, err := registry.GetExecutor("create-test")
        require.NoError(t, err)
        assert.True(t, createNodeCalled)
        assert.Equal(t, mockExecutor, executor)
    })

    t.Run("should fail to create node with non-existent type", func(t *testing.T) {
        registry := NewNodeRegistry()

        _, err := registry.GetExecutor("non-existent")
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "not found")
    })
}

// MockNodeExecutor is a mock implementation of NodeExecutor for testing
type MockNodeExecutor struct{}

func (m *MockNodeExecutor) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, nil
}

func (m *MockNodeExecutor) Validate() error {
    return nil
}

func (m *MockNodeExecutor) GetConfig() map[string]interface{} {
    return map[string]interface{}{}
}

func (m *MockNodeExecutor) SetConfig(config map[string]interface{}) error {
    return nil
}

func TestPortStruct(t *testing.T) {
    t.Run("should create port with all fields", func(t *testing.T) {
        port := Port{
            ID:          "port-1",
            Name:        "Port 1",
            Description: "First port",
            Required:    true,
        }

        assert.Equal(t, "port-1", port.ID)
        assert.Equal(t, "Port 1", port.Name)
        assert.Equal(t, "First port", port.Description)
        assert.True(t, port.Required)
    })
}

func TestPropertyStruct(t *testing.T) {
    t.Run("should create property with all fields", func(t *testing.T) {
        min := float64(0)
        max := float64(100)
        property := Property{
            Type:        "string",
            Description: "A string property",
            Default:     "default",
            Enum:        []string{"option1", "option2"},
            Min:         &min,
            Max:         &max,
            Pattern:     "^[a-z]+$",
        }

        assert.Equal(t, "string", property.Type)
        assert.Equal(t, "A string property", property.Description)
        assert.Equal(t, "default", property.Default)
        assert.Equal(t, []string{"option1", "option2"}, property.Enum)
        assert.Equal(t, &min, property.Min)
        assert.Equal(t, &max, property.Max)
        assert.Equal(t, "^[a-z]+$", property.Pattern)
    })
}

func TestSchemaStruct(t *testing.T) {
    t.Run("should create schema with properties", func(t *testing.T) {
        schema := Schema{
            Properties: map[string]Property{
                "prop1": {Type: "string", Default: "value1"},
                "prop2": {Type: "number", Default: float64(42)},
            },
            Required: []string{"prop1"},
        }

        assert.Len(t, schema.Properties, 2)
        assert.Equal(t, "string", schema.Properties["prop1"].Type)
        assert.Equal(t, "number", schema.Properties["prop2"].Type)
        assert.Contains(t, schema.Required, "prop1")
    })
}

func TestNodeMetadataStruct(t *testing.T) {
    t.Run("should create node metadata with all fields", func(t *testing.T) {
        metadata := NodeMetadata{
            ID:          "test-node",
            Type:        "function",
            Name:        "Test Node",
            Description: "A test node",
            Category:    "function",
            Inputs:      []Port{{ID: "input1", Name: "Input 1"}},
            Outputs:     []Port{{ID: "output1", Name: "Output 1"}},
            ConfigSchema: Schema{
                Properties: map[string]Property{
                    "prop1": {Type: "string"},
                },
            },
            Icon: "test-icon",
            Tags: []string{"test", "node"},
        }

        assert.Equal(t, "test-node", metadata.ID)
        assert.Equal(t, "function", metadata.Type)
        assert.Equal(t, "Test Node", metadata.Name)
        assert.Equal(t, "A test node", metadata.Description)
        assert.Equal(t, "function", metadata.Category)
        assert.Len(t, metadata.Inputs, 1)
        assert.Len(t, metadata.Outputs, 1)
        assert.Len(t, metadata.ConfigSchema.Properties, 1)
        assert.Equal(t, "test-icon", metadata.Icon)
        assert.Equal(t, []string{"test", "node"}, metadata.Tags)
    })
}

func TestNodeRegistryRegisterFactory(t *testing.T) {
    t.Run("should register node using factory method", func(t *testing.T) {
        registry := NewNodeRegistry()

        metadata := NodeMetadata{
            ID:          "factory-test",
            Name:        "Factory Test Node",
            Description: "A node registered via factory",
            Category:    "test",
        }

        factory := func() NodeExecutor {
            return &MockNodeExecutor{}
        }

        err := registry.RegisterFactory("factory-test", factory, metadata)
        require.NoError(t, err)

        // Verify the node was registered
        retrieved, err := registry.GetMetadata("factory-test")
        require.NoError(t, err)
        assert.Equal(t, metadata.ID, retrieved.ID)
        assert.Equal(t, metadata.Name, retrieved.Name)
    })
}

func TestNodeRegistryInitializeNode(t *testing.T) {
    t.Run("should initialize node with configuration", func(t *testing.T) {
        registry := NewNodeRegistry()

        config := map[string]interface{}{
            "testKey": "testValue",
        }

        mockExecutor := &MockNodeExecutor{}
        node := &Node{
            Type: "init-test",
            Metadata: NodeMetadata{
                ID:   "init-test",
                Name: "Init Test",
            },
            Factory: func() NodeExecutor {
                return mockExecutor
            },
        }

        registry.RegisterNode(node)

        executor, err := registry.InitializeNode("init-test", config)
        require.NoError(t, err)
        assert.NotNil(t, executor)
    })

    t.Run("should fail to initialize non-existent node type", func(t *testing.T) {
        registry := NewNodeRegistry()

        _, err := registry.InitializeNode("non-existent", map[string]interface{}{})
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "not found")
    })
}

func TestNodeRegistryGetNodesByCategory(t *testing.T) {
    t.Run("should return nodes filtered by category", func(t *testing.T) {
        registry := NewNodeRegistry()

        // Register nodes with different categories
        registry.RegisterNode(&Node{
            Type: "input-node",
            Metadata: NodeMetadata{
                ID:       "input-node",
                Name:     "Input Node",
                Category: "input",
            },
            Factory: func() NodeExecutor { return &MockNodeExecutor{} },
        })
        registry.RegisterNode(&Node{
            Type: "output-node",
            Metadata: NodeMetadata{
                ID:       "output-node",
                Name:     "Output Node",
                Category: "output",
            },
            Factory: func() NodeExecutor { return &MockNodeExecutor{} },
        })
        registry.RegisterNode(&Node{
            Type: "function-node",
            Metadata: NodeMetadata{
                ID:       "function-node",
                Name:     "Function Node",
                Category: "function",
            },
            Factory: func() NodeExecutor { return &MockNodeExecutor{} },
        })

        // Get input nodes
        inputNodes := registry.GetNodesByCategory("input")
        assert.Len(t, inputNodes, 1)
        assert.Equal(t, "input-node", inputNodes[0].ID)

        // Get function nodes
        functionNodes := registry.GetNodesByCategory("function")
        assert.Len(t, functionNodes, 1)
        assert.Equal(t, "function-node", functionNodes[0].ID)

        // Get non-existent category
        emptyNodes := registry.GetNodesByCategory("non-existent")
        assert.Len(t, emptyNodes, 0)
    })
}
