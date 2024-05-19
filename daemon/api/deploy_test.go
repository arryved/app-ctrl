//go:build !integration

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

func TestDeployHandlerSucceeded(t *testing.T) {
	// setup
	assert := assert.New(t)
	statusCache := model.NewStatusCache()
	deployCache := model.NewDeployCache()
	handler := http.HandlerFunc(NewConfiguredHandlerDeploy(getMockConfig(), statusCache, deployCache))

	// background client request using a test handler + responder pair
	responder := httptest.NewRecorder()
	result := DeployResult{}
	req, err := http.NewRequest("GET", "/deploy?app=arryved-api&version=1.2.3", nil)
	bgClientCh := make(chan error, 1)
	go func() {
		handler.ServeHTTP(responder, req)
		bgClientCh <- json.Unmarshal(responder.Body.Bytes(), &result)
	}()

	// check that deploy get populated as expected
	waitForDeploy(deployCache, "arryved-api")
	assert.Greater(deployCache.GetDeploys()["arryved-api"].RequestedAt, int64(0))
	assert.Equal(int64(0), deployCache.GetDeploys()["arryved-api"].CompletedAt)
	assert.Equal("1.2.3", deployCache.GetDeploys()["arryved-api"].Version)

	// mark the deploy as started and completed with nil error
	assert.True(deployCache.MarkDeployStart("arryved-api"))
	assert.True(deployCache.MarkDeployComplete("arryved-api", nil))

	// simulate intended version convergence
	intendedVersion, _ := model.ParseVersion("1.2.3")
	statusCache.SetStatuses(map[string]model.Status{
		"arryved-api": {
			Versions: model.Versions{
				Installed: &intendedVersion,
				Running:   &intendedVersion,
			},
		},
	})

	// block until the request returns, then check
	err = <-bgClientCh  // wait until response is back and unmarshaled
	assert.NoError(err) // unmarshal of response succeeded
	assert.Equal(200, responder.Code)
	assert.Equal(200, result.Code)
	assert.Equal("arryved-api", result.State.App)
	assert.Equal("1.2.3", result.State.Version)
	assert.Greater(result.State.CompletedAt, int64(0))
	assert.Greater(result.State.RequestedAt, int64(0))
	assert.Greater(result.State.StartedAt, int64(0))
}

func TestDeployHandlerFailed(t *testing.T) {
	// setup
	assert := assert.New(t)
	statusCache := model.NewStatusCache()
	deployCache := model.NewDeployCache()
	handler := http.HandlerFunc(NewConfiguredHandlerDeploy(getMockConfig(), statusCache, deployCache))

	// background client request using a test handler + responder pair
	responder := httptest.NewRecorder()
	result := DeployResult{}
	req, err := http.NewRequest("GET", "/deploy?app=arryved-api&version=1.2.3", nil)

	bgClientCh := make(chan error, 1)
	go func() {
		handler.ServeHTTP(responder, req)
		bgClientCh <- json.Unmarshal(responder.Body.Bytes(), &result)
	}()

	// check that deploy get populated as expected
	waitForDeploy(deployCache, "arryved-api")
	assert.Greater(deployCache.GetDeploys()["arryved-api"].RequestedAt, int64(0))
	assert.Equal(int64(0), deployCache.GetDeploys()["arryved-api"].CompletedAt)
	assert.Equal("1.2.3", deployCache.GetDeploys()["arryved-api"].Version)

	// mark the deploy as started, completed with error
	assert.True(deployCache.MarkDeployStart("arryved-api"))
	assert.True(deployCache.MarkDeployComplete("arryved-api", fmt.Errorf("apt update failed")))

	// block until the request returns, then check
	err = <-bgClientCh  // wait until response is back and unmarshaled
	assert.NoError(err) // unmarshal of response succeeded
	assert.Equal(500, responder.Code)
	assert.Equal(500, result.Code)
	assert.Contains(result.Err, "apt update failed")
}

func TestDeployHandlerConvergeFailed(t *testing.T) {
	// setup
	assert := assert.New(t)
	statusCache := model.NewStatusCache()
	deployCache := model.NewDeployCache()
	handler := http.HandlerFunc(NewConfiguredHandlerDeploy(getMockConfig(), statusCache, deployCache))

	// background client request using a test handler + responder pair
	responder := httptest.NewRecorder()
	result := DeployResult{}
	req, err := http.NewRequest("GET", "/deploy?app=arryved-api&version=1.2.3", nil)
	bgClientCh := make(chan error, 1)
	go func() {
		handler.ServeHTTP(responder, req)
		bgClientCh <- json.Unmarshal(responder.Body.Bytes(), &result)
	}()

	// check that deploy get populated as expected
	waitForDeploy(deployCache, "arryved-api")
	assert.Greater(deployCache.GetDeploys()["arryved-api"].RequestedAt, int64(0))
	assert.Equal(int64(0), deployCache.GetDeploys()["arryved-api"].CompletedAt)
	assert.Equal("1.2.3", deployCache.GetDeploys()["arryved-api"].Version)

	// mark the deploy as started and completed with nil error
	assert.True(deployCache.MarkDeployStart("arryved-api"))
	assert.True(deployCache.MarkDeployComplete("arryved-api", nil))

	// simulate *failed* version convergence
	intendedVersion, _ := model.ParseVersion("1.2.3")
	oldVersion, _ := model.ParseVersion("1.2.1")
	statusCache.SetStatuses(map[string]model.Status{
		"arryved-api": {
			Versions: model.Versions{
				Installed: &intendedVersion,
				Running:   &oldVersion,
			},
		},
	})

	// block until the request returns, then check that timeout occurred waiting for converge
	err = <-bgClientCh  // wait until response is back and unmarshaled
	assert.NoError(err) // unmarshal of response succeeded
	assert.Equal(408, responder.Code)
	assert.Equal(408, result.Code)
	assert.Nil(result.State)
}

func getMockConfig() *config.Config {
	// mock config, change varz port to match mock listener
	// background client request using a test handler + responser pair
	cfg := config.Load("../config/mock-config.yml")
	cfg.AptPath = "./test_objects/mock_apt"
	return cfg
}

func waitForDeploy(cache *model.DeployCache, app string) {
	for {
		time.Sleep(10 * time.Millisecond)
		if cache.GetDeploys()[app].Version != "" {
			break
		}
	}
}
