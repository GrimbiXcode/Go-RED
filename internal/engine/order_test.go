package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlowOrder_SortsTabsAndAppendsNewFlows(t *testing.T) {
	engine := createTestEngine()

	second := NewFlow("b", "B")
	second.Order = 2
	first := NewFlow("a", "A")
	first.Order = 1
	require.NoError(t, engine.AddFlow(second))
	require.NoError(t, engine.AddFlow(first))

	created, err := engine.CreateFlow("c", "C", "")
	require.NoError(t, err)
	assert.Equal(t, 3, created.Order, "a new flow goes after every existing one")

	var ids []string
	for _, flow := range engine.GetAllFlows() {
		ids = append(ids, flow.ID)
	}
	assert.Equal(t, []string{"a", "b", "c"}, ids)

	_, err = engine.UpdateFlow("c", func(f *Flow) error {
		f.Order = 0
		return nil
	})
	require.NoError(t, err)
	ids = nil
	for _, flow := range engine.GetAllFlows() {
		ids = append(ids, flow.ID)
	}
	assert.Equal(t, []string{"c", "a", "b"}, ids)
}
