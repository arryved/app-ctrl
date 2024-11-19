package gce

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	assert := assert.New(t)
	client := NewClient("dev")

	client.GetInstancesForCluster("arryved-api", "default", "default")

	assert.NotNil(client)
}
