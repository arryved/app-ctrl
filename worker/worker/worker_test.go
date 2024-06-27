//go:build integration
// +build integration

package worker

import (
	"github.com/stretchr/testify/assert"

	apiconfig "github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/api/queue"
	"github.com/arryved/app-ctrl/worker/config"
	"github.com/arryved/app-ctrl/worker/gce"

	"testing"
)

// TODO: mock k8s to isolate
func TestWorkerGKE(t *testing.T) {
	// setup
	assert := assert.New(t)
	cfg := config.Load("../config/mock-config.yml")
	client, err := queue.NewClient(cfg.Queue)
	assert.Nil(err)

	// worker object can be created
	jobQueue := queue.NewQueue(cfg.Queue, client)
	worker := New(cfg, jobQueue, nil)
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

// TODO: mock gce to isolate
func TestWorkerGCE(t *testing.T) {
	// setup
	assert := assert.New(t)
	cfg := config.Load("../config/mock-config.yml")
	client, err := queue.NewClient(cfg.Queue)
	assert.Nil(err)

	// worker object can be created
	jobQueue := queue.NewQueue(cfg.Queue, client)
	compute := gce.NewClient("dev", "central")
	worker := New(cfg, jobQueue, compute)
	assert.NotNil(worker)

	// the worker can process a Job object
	job := queue.Job{
		Id:        "xyz",
		Action:    "DEPLOY",
		Principal: "example@arryved.com",
		Request: &queue.DeployJobRequest{
			Cluster: apiconfig.Cluster{
				Id: apiconfig.ClusterId{
					App:     "arryved-merchant",
					Region:  "central",
					Variant: "default",
				},
				Kind:    "online",
				Repo:    "arryved-apt",
				Runtime: "GCE",
			},
			Version: "2.44.0",
		},
	}
	result, err := worker.ProcessJob(&job)
	assert.Nil(err)
	assert.NotNil(result)
	assert.Equal("COMPLETE", result.ActionStatus)
	assert.Equal("HEALTHY", result.ClusterStatus)
}
