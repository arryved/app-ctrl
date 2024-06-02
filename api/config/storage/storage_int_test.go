//go:build integration

package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGCPStorageClient(t *testing.T) {
	assert := assert.New(t)
	client, err := New(context.Background())
	assert.Nil(err)
	assert.NotNil(client)

	objects, err := client.ListObjects("arryved-app-control-config")
	assert.Nil(err)
	assert.Greater(len(objects), 0)

	bytes, err := objects[0].GetContents()
	assert.Nil(err)
	assert.Greater(len(bytes), 0)
}
