package gce

import (
	"github.com/stretchr/testify/assert"

	//"github.com/arryved/app-ctrl/api/queue"
	//"github.com/arryved/app-ctrl/worker/config"

	"testing"
)

func TestClient(t *testing.T) {
	assert := assert.New(t)
	client := NewClient("dev", "central")

	client.GetInstancesForCluster("arryved-api", "default")

	assert.NotNil(client)
}
