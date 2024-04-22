package worker

import (
	"github.com/stretchr/testify/assert"

	apiconfig "github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/api/queue"
	"github.com/arryved/app-ctrl/worker/config"

	"testing"
)

// TODO: mock k8s to isolate
func TestWorker(t *testing.T) {
	// setup
	assert := assert.New(t)
	cfg := config.Load("../config/mock-config.yml")
	client, err := queue.NewClient(cfg.Queue)
	assert.Nil(err)

	// worker object can be created
	jobQueue := queue.NewQueue(cfg.Queue, client)
	worker := New(cfg, jobQueue)
	assert.NotNil(worker)

	// the worker can process a Job object
	job := queue.Job{
		Id:        "xyz",
		Action:    "DEPLOY",
		Principal: "example@arryved.com",
		Request: &queue.DeployJobRequest{
			Cluster: apiconfig.Cluster{
				Id: apiconfig.ClusterId{
					App:     "pay",
					Region:  "central",
					Variant: "default",
				},
				Kind:    "online",
				Repo:    "docker-product",
				Runtime: "GKE",
			},
			Version: "0.0.3",
		},
	}
	result, err := worker.ProcessJob(&job)
	assert.Nil(err)
	assert.NotNil(result)
	assert.Equal("COMPLETE", result.ActionStatus)
	assert.Equal("HEALTHY", result.ClusterStatus)
}
