# Performance

Measured, not promised: the numbers below come from the benchmarks in
`internal/engine/bench_test.go`, which push messages through a real
`FlowEngine` running the real node implementations and count them at the
far end of the flow. Re-run them before repeating any throughput figure:

```bash
go test ./internal/engine -run '^$' -bench . -benchtime 20000x -count 3
```

## Results

Machine: 4 vCPU Intel Xeon @ 2.80 GHz, Linux amd64, Go 1.25, race detector
off, `-benchtime 20000x -count 3` (2026-09-20, commit of Phase 6).

| Benchmark | Flow | Result (per input message) |
|---|---|---|
| `BenchmarkFunctionChain` | source → function (JavaScript, `msg.count++`) → sink | 87 000 – 93 000 msg/s, ≈ 11 µs |
| `BenchmarkInjectFunctionDebug` | source → function → debug (debug sidebar ring buffer and events) | 78 000 – 82 000 msg/s, ≈ 12 µs |
| `BenchmarkSwitchFanout` | source → switch with three matching rules → three sinks | 60 000 – 62 000 msg/s in, i.e. ≈ 180 000 deliveries/s |
| `BenchmarkSplitJoin` | source → split (10 elements) → join → sink | 22 500 – 23 500 arrays/s, i.e. ≈ 230 000 part messages/s |

The JavaScript function node (goja) accounts for most of the cost wherever
it appears; a flow of plain Go nodes moves several hundred thousand
messages per second per core. The old README claim of "> 100,000
messages/second" is therefore true for simple Go-node flows and false for
flows with a function node on one core; the table is what to quote.

## How the engine spends its time

Per hop the engine does one queue send (a buffered channel per flow), one
goroutine start bounded by the flow's execution slots, one message clone
(payload map and metadata), one context with timeout, and the counters
behind `/metrics` and `flow:metrics`. There is no engine-wide worker pool
and no lock per message on the hot path; the message log behind
`GET /api/messages` is off unless `-message-log` is set.

The benchmarks are closed-loop: the producer keeps at most 256 messages
outstanding, because the engine drops rather than blocks when a flow queue
(`-max-messages`, default 1000) overflows. Bursts larger than the queue lose
messages by design (`gored_messages_dropped_total` counts them); size the
queue and the concurrency cap (`-max-inflight`, or the flow's
`maxConcurrency`) for the burst you expect.

## Reading the numbers

- `msg/s` is input messages per second through the whole flow, measured
  wall-clock from the first inject to the last arrival; `ns/op` is the
  same figure per message.
- Nodes that wait (delay, http request, tcp request, exec) hold a goroutine
  and an execution slot for the time they wait, so throughput on such flows
  is bounded by `maxConcurrency / wait time`, not by the engine.
- `go test -race` roughly halves every figure; CI runs the benchmarks for
  one iteration only, to catch breakage, not to measure.
