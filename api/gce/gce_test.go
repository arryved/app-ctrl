package gce

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestClient(t *testing.T) {
	assert := assert.New(t)
	client := NewClient("dev", "central")

	client.GetRegionAppControlInstances()

	assert.NotNil(client)
}
