package ahasendmcp

import (
	"context"
	"os"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAhaMLReferenceIsListed(t *testing.T) {
	// The reference is listed with unknown scopes (nil), and with no scopes,
	// which list no tool.
	for _, scopes := range [][]string{nil, {}} {
		opts := testOptions(t, newFakeAPI(t))
		opts.Scopes = scopes
		cs := connect(t, opts)

		res, err := cs.ListResources(context.Background(), nil)
		require.NoError(t, err)
		require.Len(t, res.Resources, 1)
		r := res.Resources[0]
		assert.Equal(t, "ahasend://docs/ahaml", r.URI)
		assert.Equal(t, "ahaml", r.Name)
		assert.Equal(t, "AhaML reference", r.Title)
		assert.Equal(t, "text/markdown", r.MIMEType)
		assert.Equal(t, int64(len(ahamlReference)), r.Size)
		assert.NotEmpty(t, r.Description)
	}
}

func TestAhaMLReferenceReadsAsTheEmbeddedCopy(t *testing.T) {
	cs := connect(t, testOptions(t, newFakeAPI(t)))

	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "ahasend://docs/ahaml"})
	require.NoError(t, err)
	require.Len(t, res.Contents, 1)
	c := res.Contents[0]
	assert.Equal(t, "ahasend://docs/ahaml", c.URI)
	assert.Equal(t, "text/markdown", c.MIMEType)

	file, err := os.ReadFile("ahaml-reference.md")
	require.NoError(t, err)
	require.NotEmpty(t, file)
	assert.Equal(t, string(file), c.Text, "the resource is the embedded file, byte for byte")
	assert.Equal(t, ahamlReference, c.Text)
}

func TestUnknownResourceIsNotFound(t *testing.T) {
	cs := connect(t, testOptions(t, newFakeAPI(t)))
	_, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "ahasend://docs/other"})
	require.Error(t, err)
}
