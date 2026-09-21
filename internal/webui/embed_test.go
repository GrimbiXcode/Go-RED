package webui

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDist(t *testing.T) {
	fsys, ok := Dist()
	require.NotNil(t, fsys)
	_, err := fs.Stat(fsys, "index.html")
	assert.Equal(t, err == nil, ok, "ok mirrors the presence of index.html")
}
