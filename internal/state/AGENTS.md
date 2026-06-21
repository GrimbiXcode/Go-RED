# Go-RED State Manager Guidelines

This file contains **state-specific** guidelines for flow persistence in the `internal/state/` directory.

---

## Package Overview

The `state/` package provides **persistence abstraction** for Go-RED flows. It enables:

1. **Flow Storage**: Save, load, and delete flows
2. **Backend Abstraction**: Support multiple storage backends (file, database)
3. **State Management**: Interface for the flow engine to persist flows

---

## Architecture

### Core Interface

The package defines a **StateManager** interface that all backends must implement:

```go
type StateManager interface {
    // Save a flow
    SaveFlow(flow *engine.Flow) error
    
    // Load a specific flow by ID
    LoadFlow(flowID string) (*engine.Flow, error)
    
    // Load all flows
    LoadAllFlows() ([]*engine.Flow, error)
    
    // Delete a flow
    DeleteFlow(flowID string) error
}
```

### Current Implementation

```
state/
└── manager.go          # StateManager interface + FileStateManager
```

**FileStateManager**: Default implementation that stores flows as JSON files in `data/flows/`.

---

## FileStateManager

The default state manager implementation uses the file system:

```go
type FileStateManager struct {
    baseDir string // e.g., "data/flows/"
    mu      sync.RWMutex
}
```

### File Structure

```
data/flows/
├── flow-1e03a984-4b34-4068-b342-d1decc29c56d.json
├── flow-2ab5c8d1-e3f4-5a6b-7c8d-9e0f1a2b3c4d.json
└── ...
```

### File Format

Each flow is stored as a JSON file with the flow ID as the filename:

```json
{
  "id": "flow-1e03a984-4b34-4068-b342-d1decc29c56d",
  "name": "My Flow",
  "description": "A test flow",
  "nodes": {
    "node-1": {
      "id": "node-1",
      "type": "inject",
      "name": "Inject",
      "x": 100,
      "y": 200,
      "config": {}
    }
  },
  "connections": [],
  "config": {
    "timeout": 30,
    "maxConcurrency": 10,
    "environment": {},
    "retryPolicy": {
      "maxRetries": 3,
      "backoff": 1,
      "maxBackoff": 30,
      "retryOn": []
    }
  },
  "status": "inactive",
  "createdAt": "2026-06-21T10:00:00Z",
  "updatedAt": "2026-06-21T10:00:00Z",
  "version": "1.0.0"
}
```

---

## Development Guidelines

### Adding a New Backend

To add a new storage backend (e.g., PostgreSQL, MongoDB):

1. **Create a new file**: `internal/state/[backend]_manager.go`

2. **Implement the StateManager interface**:

```go
type PostgreSQLStateManager struct {
    db *sql.DB
}

func NewPostgreSQLStateManager(connectionString string) (*PostgreSQLStateManager, error) {
    db, err := sql.Open("postgres", connectionString)
    if err != nil {
        return nil, err
    }
    return &PostgreSQLStateManager{db: db}, nil
}

func (m *PostgreSQLStateManager) SaveFlow(flow *engine.Flow) error {
    // Implement using SQL INSERT/UPDATE
}

func (m *PostgreSQLStateManager) LoadFlow(flowID string) (*engine.Flow, error) {
    // Implement using SQL SELECT
}

func (m *PostgreSQLStateManager) LoadAllFlows() ([]*engine.Flow, error) {
    // Implement using SQL SELECT *
}

func (m *PostgreSQLStateManager) DeleteFlow(flowID string) error {
    // Implement using SQL DELETE
}
```

3. **Add to main.go**:

```go
// Add flag for backend selection
var backend string
flag.StringVar(&backend, "state-backend", "file", "State backend: file, postgres")

// Initialize appropriate backend
var stateManager state.StateManager
var err error

switch backend {
case "postgres":
    stateManager, err = state.NewPostgreSQLStateManager(config.DatabaseURL)
case "file":
    stateManager, err = state.NewFileStateManager(config.DataDir)
default:
    return fmt.Errorf("unknown state backend: %s", backend)
}
```

---

## FileStateManager Implementation

### Public Methods

#### NewFileStateManager
```go
func NewFileStateManager(baseDir string) (*FileStateManager, error)
```
Creates a new FileStateManager with the specified base directory.
- Creates the directory if it doesn't exist
- Validates directory is writable

#### SaveFlow
```go
func (m *FileStateManager) SaveFlow(flow *engine.Flow) error
```
Saves a flow to a JSON file:
1. Acquires write lock
2. Marshals flow to JSON
3. Writes to file atomically (write to temp, then rename)
4. Updates UpdatedAt timestamp

#### LoadFlow
```go
func (m *FileStateManager) LoadFlow(flowID string) (*engine.Flow, error)
```
Loads a flow from a JSON file:
1. Acquires read lock
2. Reads file content
3. Unmarshals JSON to Flow struct
4. Validates flow structure

#### LoadAllFlows
```go
func (m *FileStateManager) LoadAllFlows() ([]*engine.Flow, error)
```
Loads all flows from the directory:
1. Reads all `.json` files from baseDir
2. Parses each file as a Flow
3. Returns slice of all flows
4. Skips malformed files (logs error)

#### DeleteFlow
```go
func (m *FileStateManager) DeleteFlow(flowID string) error
```
Deletes a flow file:
1. Acquires write lock
2. Deletes the corresponding `.json` file
3. Returns error if file doesn't exist

---

## Error Handling

### Standard Errors

```go
var (
    ErrFlowNotFound      = errors.New("flow not found")
    ErrInvalidFlow       = errors.New("invalid flow data")
    ErrFlowAlreadyExists = errors.New("flow already exists")
    ErrDirectoryNotFound = errors.New("directory not found")
    ErrPermissionDenied  = errors.New("permission denied")
    ErrStorageError      = errors.New("storage error")
)
```

### Error Wrapping

Always wrap errors with context:

```go
func (m *FileStateManager) SaveFlow(flow *engine.Flow) error {
    filename := m.flowPath(flow.ID)
    
    data, err := json.MarshalIndent(flow, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal flow %s: %w", flow.ID, err)
    }
    
    if err := os.WriteFile(filename, data, 0644); err != nil {
        return fmt.Errorf("failed to write flow %s: %w", flow.ID, err)
    }
    
    return nil
}
```

---

## Thread Safety

### Locking Strategy

- **Read Operations** (`LoadFlow`, `LoadAllFlows`): Use `RLock()` for concurrent reads
- **Write Operations** (`SaveFlow`, `DeleteFlow`): Use `Lock()` for exclusive access
- **File Operations**: Atomic writes using temp file + rename

### Concurrency Considerations

- Multiple goroutines can safely call state manager methods
- File system operations are the bottleneck, not the Go code
- For high-throughput scenarios, consider:
  - Batch writes
  - Write-behind caching
  - Async persistence

---

## Testing

### Unit Tests

```go
func TestFileStateManager_SaveLoad(t *testing.T) {
    // Create temp directory
    tmpDir := t.TempDir()
    
    // Create state manager
    sm, err := state.NewFileStateManager(tmpDir)
    require.NoError(t, err)
    
    // Create test flow
    flow := &engine.Flow{
        ID:   "test-flow",
        Name: "Test Flow",
        Nodes: map[string]*engine.Node{
            "n1": {ID: "n1", Type: "inject"},
        },
    }
    
    // Save flow
    err = sm.SaveFlow(flow)
    require.NoError(t, err)
    
    // Verify file exists
    filepath := filepath.Join(tmpDir, "test-flow.json")
    _, err = os.Stat(filepath)
    require.NoError(t, err)
    
    // Load flow
    loaded, err := sm.LoadFlow("test-flow")
    require.NoError(t, err)
    assert.Equal(t, flow.ID, loaded.ID)
    assert.Equal(t, flow.Name, loaded.Name)
}

func TestFileStateManager_LoadAll(t *testing.T) {
    tmpDir := t.TempDir()
    sm, _ := state.NewFileStateManager(tmpDir)
    
    // Save multiple flows
    for i := 0; i < 5; i++ {
        flow := &engine.Flow{
            ID:   fmt.Sprintf("flow-%d", i),
            Name: fmt.Sprintf("Flow %d", i),
        }
        sm.SaveFlow(flow)
    }
    
    // Load all
    flows, err := sm.LoadAllFlows()
    require.NoError(t, err)
    assert.Len(t, flows, 5)
}

func TestFileStateManager_Delete(t *testing.T) {
    tmpDir := t.TempDir()
    sm, _ := state.NewFileStateManager(tmpDir)
    
    flow := &engine.Flow{ID: "to-delete"}
    sm.SaveFlow(flow)
    
    // Verify exists
    _, err := sm.LoadFlow("to-delete")
    require.NoError(t, err)
    
    // Delete
    err = sm.DeleteFlow("to-delete")
    require.NoError(t, err)
    
    // Verify gone
    _, err = sm.LoadFlow("to-delete")
    assert.Error(t, err)
    assert.Equal(t, state.ErrFlowNotFound, err)
}
```

### Integration Tests

```go
func TestStateManager_WithEngine(t *testing.T) {
    tmpDir := t.TempDir()
    sm, _ := state.NewFileStateManager(tmpDir)
    
    // Create engine with state manager
    registry := registry.GetGlobalRegistry()
    engine := NewFlowEngine(DefaultEngineConfig(), registry)
    engine.SetStateManager(sm)
    
    // Create and save flow through engine
    flow, err := engine.CreateFlow("test-flow", "Test Flow")
    require.NoError(t, err)
    
    // Flow should be saved automatically
    files, _ := os.ReadDir(tmpDir)
    assert.Len(t, files, 1)
    
    // Load all flows through engine
    err = engine.LoadAllFlows()
    require.NoError(t, err)
    
    // Flow should be loaded
    allFlows := engine.GetAllFlows()
    assert.Len(t, allFlows, 1)
}
```

### Error Tests

```go
func TestFileStateManager_Errors(t *testing.T) {
    // Non-existent directory
    _, err := state.NewFileStateManager("/nonexistent/path")
    assert.Error(t, err)
    
    // Load non-existent flow
    tmpDir := t.TempDir()
    sm, _ := state.NewFileStateManager(tmpDir)
    _, err = sm.LoadFlow("nonexistent")
    assert.Equal(t, state.ErrFlowNotFound, err)
    
    // Delete non-existent flow
    err = sm.DeleteFlow("nonexistent")
    assert.Error(t, err)
}
```

---

## Performance Optimization

### Current Implementation

The FileStateManager uses:
- **JSON serialization**: Standard `encoding/json`
- **Atomic writes**: Write to temp file, then rename
- **Pretty printing**: `json.MarshalIndent` for human-readable files
- **Sync I/O**: Blocking file operations

### Optimization Opportunities

1. **Batch Writes**: Save multiple flows in a single operation
2. **Async Writes**: Background persistence to avoid blocking
3. **Write-Behind Cache**: Cache writes and flush periodically
4. **Binary Format**: Use more compact serialization (e.g., Protocol Buffers)
5. **Compression**: Gzip large flow files
6. **Index File**: Maintain index.json with all flow metadata for faster listing

### Benchmarking

```go
func BenchmarkFileStateManager_SaveFlow(b *testing.B) {
    tmpDir := b.TempDir()
    sm, _ := state.NewFileStateManager(tmpDir)
    
    flow := &engine.Flow{
        ID:   "bench-flow",
        Name: "Benchmark Flow",
        Nodes: make(map[string]*engine.Node),
    }
    
    // Add many nodes
    for i := 0; i < 100; i++ {
        flow.Nodes[fmt.Sprintf("node-%d", i)] = &engine.Node{
            ID:   fmt.Sprintf("node-%d", i),
            Type: "function",
            Config: map[string]interface{}{
                "function": "return msg;",
            },
        }
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        flow.ID = fmt.Sprintf("bench-flow-%d", i)
        sm.SaveFlow(flow)
    }
}

func BenchmarkFileStateManager_LoadAll(b *testing.B) {
    tmpDir := b.TempDir()
    sm, _ := state.NewFileStateManager(tmpDir)
    
    // Create many flows
    for i := 0; i < 100; i++ {
        flow := &engine.Flow{
            ID:   fmt.Sprintf("flow-%d", i),
            Name: fmt.Sprintf("Flow %d", i),
        }
        sm.SaveFlow(flow)
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        sm.LoadAllFlows()
    }
}
```

---

## Backup and Recovery

### Manual Backup

```bash
# Backup all flows
tar -czvf flow-backup-$(date +%Y%m%d).tar.gz data/flows/

# Restore from backup
tar -xzvf flow-backup-20260621.tar.gz -C data/
```

### Automated Backup

Consider adding a backup endpoint:

```go
func handleBackupFlows(w http.ResponseWriter, r *http.Request, sm state.StateManager) {
    flows, err := sm.(*FileStateManager).LoadAllFlows()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Create backup archive
    var buf bytes.Buffer
    zw := zip.NewWriter(&buf)
    
    for _, flow := range flows {
        f, err := zw.Create(flow.ID + ".json")
        if err != nil {
            continue
        }
        data, _ := json.Marshal(flow)
        f.Write(data)
    }
    
    zw.Close()
    
    w.Header().Set("Content-Type", "application/zip")
    w.Header().Set("Content-Disposition", "attachment; filename=flows-backup.zip")
    w.Write(buf.Bytes())
}
```

---

## Migration Between Backends

### Export from File Backend

```go
func ExportFlowsFromFileBackend(fileDir string) ([]*engine.Flow, error) {
    sm, err := state.NewFileStateManager(fileDir)
    if err != nil {
        return nil, err
    }
    return sm.LoadAllFlows()
}
```

### Import to Database Backend

```go
func ImportFlowsToDatabaseBackend(dbManager state.StateManager, flows []*engine.Flow) error {
    for _, flow := range flows {
        if err := dbManager.SaveFlow(flow); err != nil {
            // Handle duplicate IDs
            if errors.Is(err, state.ErrFlowAlreadyExists) {
                log.Printf("Skipping duplicate flow: %s", flow.ID)
                continue
            }
            return err
        }
    }
    return nil
}
```

---

## Security Considerations

### File System Security

1. **Directory Permissions**: Ensure `data/flows/` has proper permissions
   ```go
   // In NewFileStateManager
   if err := os.MkdirAll(baseDir, 0750); err != nil {
       return nil, fmt.Errorf("failed to create directory: %w", err)
   }
   ```

2. **File Validation**: Validate flow files on load
   ```go
   func (m *FileStateManager) LoadFlow(flowID string) (*engine.Flow, error) {
       data, err := os.ReadFile(m.flowPath(flowID))
       if err != nil {
           return nil, err
       }
       
       var flow engine.Flow
       if err := json.Unmarshal(data, &flow); err != nil {
           return nil, fmt.Errorf("invalid JSON in flow %s: %w", flowID, err)
       }
       
       // Validate flow structure
       if err := flow.Validate(); err != nil {
           return nil, fmt.Errorf("invalid flow %s: %w", flowID, err)
       }
       
       return &flow, nil
   }
   ```

3. **Path Traversal Protection**: Prevent directory traversal attacks
   ```go
   func (m *FileStateManager) flowPath(flowID string) string {
       // Sanitize flowID to prevent path traversal
       safeID := filepath.Base(flowID)
       // Ensure only valid characters
       safeID = regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(safeID, "_")
       return filepath.Join(m.baseDir, safeID+".json")
   }
   ```

---

## Debugging

### Common Issues

#### Flow Not Saved
**Symptoms**: Flow doesn't appear in directory, not loaded on restart

**Checks:**
1. Is the state manager set on the engine?
2. Are there write permissions on the directory?
3. Is there enough disk space?
4. Are error logs showing the failure?

**Solution:**
```go
// Enable debug logging
state.VerboseLogging = true

// Check state manager is set
if engine.GetStateManager() == nil {
    log.Println("State manager not set!")
}
```

#### Flow Not Loaded
**Symptoms**: Flows don't appear after restart

**Checks:**
1. Do files exist in the data directory?
2. Are the JSON files valid?
3. Is the flow ID matching the filename?
4. Are there validation errors?

**Solution:**
```go
// Test loading manually
sm, _ := state.NewFileStateManager("data/flows/")
flows, err := sm.LoadAllFlows()
if err != nil {
    log.Printf("Load error: %v", err)
}
log.Printf("Loaded %d flows", len(flows))
```

#### Corrupted Flow File
**Symptoms**: Specific flow fails to load, others work

**Solution:**
1. Check the JSON syntax of the file
2. Validate the flow structure
3. Restore from backup

---

## Future Enhancements

### Planned Backends
- **PostgreSQL**: Full-featured relational database backend
- **MongoDB**: Document-based storage
- **SQLite**: Single-file database, good for embedded
- **Redis**: In-memory with persistence option
- **S3/Cloud Storage**: Cloud-based storage

### Advanced Features
- **Flow versioning**: Track history of flow changes
- **Collision detection**: Prevent concurrent modifications
- **Offline support**: Queue writes when offline, sync later
- **Sync across instances**: Distributed state management
- **Encryption**: Encrypt sensitive flow data at rest
- **Backup automation**: Scheduled backups with retention policies

---

## Checklist for State Manager Changes

Before committing changes to the state manager:

- [ ] All existing tests pass (`go test ./internal/state/...`)
- [ ] No race conditions (`go test -race ./internal/state/...`)
- [ ] File operations are atomic (write to temp, then rename)
- [ ] Thread safety is maintained (proper locking)
- [ ] Error handling is comprehensive
- [ ] Performance hasn't degraded (run benchmarks)
- [ ] Data migration path exists (if changing format)
- [ ] Frontend integration still works
- [ ] Documentation is updated

---

*Last updated: 2026-06-21*
*Overrides: None (extends internal/AGENTS.md and root AGENTS.md)*
