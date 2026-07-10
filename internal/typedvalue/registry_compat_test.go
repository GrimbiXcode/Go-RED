// This file (package typedvalue_test, an external test package) is a
// compile-time guard: it fails to build if registry.ContextStore ever stops
// structurally satisfying typedvalue.ContextAccessor. typedvalue must not
// import internal/registry directly (that would risk a future import cycle
// once engine/registry code starts constructing typedvalue.Resolver/
// PropertyResolver values), so this is the only place the two are checked
// against each other.
package typedvalue_test

import (
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
)

func TestContextStoreSatisfiesContextAccessor(t *testing.T) {
    var _ typedvalue.ContextAccessor = (*registry.ContextStore)(nil)
}
