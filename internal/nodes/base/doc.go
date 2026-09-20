// Package base holds the small helpers every node package under
// internal/nodes used to hand-roll for itself: reading JSON-decoded
// configuration maps with defaults (Config, Decode), coercing message
// values (ToFloat, ToString, ToBytes, CloneMap, ...), converting
// typedvalue.Value/PropertyRef to and from their config-map form, and
// extracting the context.Context / registry.NodeRuntime the engine passes
// to Execute as an interface{} (Context, Runtime, Resolvers).
//
// It is deliberately not a framework: a node keeps implementing
// registry.NodeExecutor itself and just calls these functions where it
// previously had a private copy. Nothing here changes the shape of a
// node's configuration or its metadata (docs/NEXT_LEVEL_PLAN.md, Phase 6,
// "Node-Basisklasse (nodes/base) gegen Duplikate").
package base
