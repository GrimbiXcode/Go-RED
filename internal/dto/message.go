package dto

// Message is the canonical wire representation of a message flowing through
// a flow, used by GET /api/messages and the message:log/message:send
// WebSocket messages. It mirrors internal/engine.Message's real JSON shape
// (Metadata is map[string]string, not a richly-typed object — Go cannot
// produce anything richer than that today).
type Message struct {
	ID        string                 `json:"id"`
	FlowID    string                 `json:"flowId"`
	Payload   map[string]interface{} `json:"payload"`
	Metadata  map[string]string      `json:"metadata"`
	Path      []string               `json:"path"`
	Timestamp string                 `json:"timestamp"`
}
