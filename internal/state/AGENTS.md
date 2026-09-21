# internal/state — flow persistence

`state` implements `engine.StateManager` (defined in `internal/engine/engine.go`) on
the file system. It imports `internal/engine` for `engine.Flow`, `engine.ErrFlowNotFound`
and `engine.ValidateFlowID`; nothing in `engine` imports it back. `FileStateManager` is
the only implementation; the engine's tests use their own in-memory one.

```go
type StateManager interface {          // engine package
    SaveFlow(flow *Flow) error
    LoadFlow(flowID string) (*Flow, error)
    LoadAllFlows() ([]*Flow, error)
    DeleteFlow(flowID string) error
}
```

## Files

| File | Contents |
|---|---|
| `manager.go` | `Options`, `DefaultOptions`, `FileStateManager`, constructors, `SaveFlow`, `LoadFlow`, `LoadAllFlows`, `DeleteFlow`, quarantine, `writeFileAtomic`, `ErrCorruptFlowFile`, `ErrSchemaVersionTooNew` |
| `backup.go` | `Backups(flowID)`, backup naming, rotation and pruning |
| `migrate.go` | `schemaVersionKey`, `currentSchemaVersion`, `migrations`, `migrateV1`, `migrate` |
| `manager_test.go`, `backup_test.go`, `migrate_test.go`, `migrate_v1_test.go` | tests |

## Layout on disk

```
<basePath>/
├── flows/<flowID>.json                    live flows
├── backups/<flowID>.<UTC stamp>.json      earlier versions, rotated by SaveFlow
└── quarantine/<flowID>.<UTC stamp>.json   files LoadAllFlows could not parse
```

`basePath` is `cmd/go-red`'s `DataDir` (`-data-dir`, `GORED_DATA_DIR`, default `data`).
`fileTimeLayout` (`20060102T150405Z`) is fixed width, so names sort chronologically.
Flow IDs pass `engine.ValidateFlowID` in `flowPath`, so an ID can never escape the
directory and never contains a dot (which makes `<flowID>.` an unambiguous backup
prefix).

A flow file is the `engine.Flow` JSON object with one extra top-level key,
`schemaVersion` (`flowFile` struct). It is the internal representation, not the wire
format of `internal/dto`: `time.Duration` fields are raw nanoseconds, `status` is the
engine enum. Do not hand-describe or hand-edit it as if it were the REST shape.

## Constructors and options

- `NewFileStateManager(basePath)` = `NewFileStateManagerWithOptions(basePath,
  DefaultOptions())`. Both create `basePath` and `basePath/flows` with `MkdirAll`; a
  missing directory is not an error, a path blocked by a file is.
- `Options{BackupKeep int, BackupMinInterval time.Duration}`; defaults 5 and 10 min.
  `BackupKeep` 0 disables backups; negative values count as 0. `cmd/go-red` fills them
  from `-backup-keep`/`-backup-interval` (`GORED_BACKUP_KEEP`, `GORED_BACKUP_INTERVAL`).
- `FileStateManager.now` is the clock for backup and quarantine names; tests replace it.

## Behaviour

**SaveFlow** (write lock): marshal `flowFile{SchemaVersion: currentSchemaVersion,
Flow}` with indentation, back up the existing file (`backupLocked`), then
`writeFileAtomic`: temp file in the same directory, `Sync`, chmod 0644, `Rename` over
the target. A crash mid-write never leaves a truncated flow. A failed backup is logged
and does not stop the save.

**Backups** (`backup.go`): skipped when there is no previous file, when `BackupKeep <= 0`,
or when the newest backup is younger than `BackupMinInterval` (SaveFlow runs on every
autosave, so without the interval a burst of keystrokes would rotate every useful backup
out). After writing, the oldest backups beyond `BackupKeep` are removed.
`Backups(flowID)` lists a flow's backups oldest first; no backups is an empty list, not
an error. `DeleteFlow` keeps backups so a deletion can be undone by hand.

**LoadFlow** (read lock): missing file → `engine.ErrFlowNotFound`; not valid JSON or not
an object → `ErrCorruptFlowFile` (file left in place); newer `schemaVersion` than this
build → `ErrSchemaVersionTooNew` (file left untouched, so a downgrade loses nothing).
Older versions are migrated in memory; the file is rewritten on the next `SaveFlow`.
`decodeFlowFile` decodes with `UseNumber`, runs `migrate`, re-encodes, then unmarshals
into `engine.Flow`; a missing `id` is filled from the file name and a nil `nodes` map is
initialised.

**LoadAllFlows**: every `*.json` in `flows/` (dot-files skipped). One bad file never
takes the others down: corrupt files are moved to `quarantine/` and logged, too-new
files are logged and skipped, other read errors are logged and skipped. The return
value only errors when the directory cannot be read.

All errors from this package are wrapped with `%w`, so callers use `errors.Is`.

## Schema versions and migrations

- `currentSchemaVersion` is 2. Files without the key (or with 0) count as version 1.
- `migrations` maps version `v` to the function that upgrades a raw
  `map[string]any` in place from `v` to `v+1`; `migrate` runs the chain from the file's
  version to the current one and stamps the new version.
- `migrateV1`: version 1 files carry the never-enforced placeholder
  `config.maxConcurrency: 100`; it becomes 0 ("engine default"). Any other value stays.
- To change the on-disk shape: bump `currentSchemaVersion`, add `migrations[old]`,
  add a test in the style of `migrate_v1_test.go`, and update the history comment in
  `migrate.go` and `docs/PROTOCOL.md` if the wire shape changes too.
- `currentSchemaVersion` and `migrations` are variables only so `migrate_test.go` can
  swap in a longer chain; production code never assigns them.

## Locking

One `sync.RWMutex` per manager: `SaveFlow`/`DeleteFlow` take the write lock,
`LoadFlow`/`LoadAllFlows`/`Backups` the read lock. `*Locked` helpers
(`loadFlowLocked`, `backupLocked`, `listBackupsLocked`) assume the caller holds it.
The engine additionally calls `SaveFlow` under its own `e.mu`, so the flow it hands in
is stable during serialization.

## Testing patterns

- `t.TempDir()` (or `os.MkdirTemp` + `RemoveAll`) per test; never the server's real
  data directory.
- `newBackupTestManager` installs a `fakeClock` on `manager.now` and advances it instead
  of sleeping through `BackupMinInterval`.
- `migrate_test.go` writes raw JSON files by hand (old versions, future versions,
  invalid `schemaVersion` values) and checks what `LoadFlow`/`LoadAllFlows` do with
  them; `TestFileStateManagerQuarantine` covers the move to `quarantine/`.
- `TestFileStateManagerConcurrentOperations` hammers save/load from goroutines; run
  with `go test -race ./internal/state/...`.
