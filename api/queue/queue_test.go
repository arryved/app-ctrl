//go:build !integration

package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/arryved/app-ctrl/api/config"
)

func TestNewQueue(t *testing.T) {
	assert := assert.New(t)
	cfg := config.Load("../config/mock-config.yml")

	queue := NewQueue(cfg.Queue, nil)

	assert.NotNil(queue)
}

// TODO this requires a live pubsub, mock to isolate
func TestEnqueueDeploy(t *testing.T) {
	assert := assert.New(t)
	cfg := config.Load("../config/mock-config.yml")
	client, err := NewClient(cfg.Queue)
	assert.Nil(err)
	assert.NotNil(client)

	queue := NewQueue(cfg.Queue, client)
	assert.NotNil(queue)

	job, err := NewJob("example@arryved.com", DeployJobRequest{
		Cluster: config.Cluster{
			Id: config.ClusterId{
				App:     "arryved-api",
				Region:  "central",
				Variant: "default",
			},
			Hosts: map[string]config.Host{
				"core-api-xyz.dev.arryved.com": {
					Canary: false,
				},
			},
			Kind:    "online",
			Repo:    "arryved-apt",
			Runtime: "GCE",
		},
		Concurrency: "1",
		Version:     "0.1.1",
	})
	assert.Nil(err)
	assert.NotNil(job)

	result, err := queue.Enqueue(job)
	assert.Nil(err)
	assert.NotNil(result)
}

// TODO this requires a live pubsub, mock to isolate
func TestDequeueDeploy(t *testing.T) {
	assert := assert.New(t)
	cfg := config.Load("../config/mock-config.yml")

	client, err := NewClient(cfg.Queue)
	assert.Nil(err)
	assert.NotNil(client)
	defer client.Close()

	queue := NewQueue(cfg.Queue, client)
	assert.NotNil(queue)

	job, err := queue.Dequeue()
	assert.Nil(err)
	assert.NotNil(job)
	assert.Equal(job.Action, "DEPLOY")
	assert.Equal(job.Principal, "example@arryved.com")
	assert.Equal(job.Request.(*DeployJobRequest).Concurrency, "1")
	assert.Equal(job.Request.(*DeployJobRequest).Version, "0.1.1")
	assert.Equal(job.Request.(*DeployJobRequest).Cluster.Id.App, "arryved-api")
	assert.Equal(job.Request.(*DeployJobRequest).Cluster.Id.Region, "central")
	assert.Equal(job.Request.(*DeployJobRequest).Cluster.Id.Variant, "default")
}
