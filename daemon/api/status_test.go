//go:build !integration

package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
	"github.com/arryved/app-ctrl/daemon/runners"
)

func TestStatusHandler(t *testing.T) {
	assert := assert.New(t)

	// mock config, change varz port to match mock listener
	cfg := config.Load("../config/mock-config.yml")
	varzPort := mockVarzListenerForStatus()
	cfg.AptPath = "./test_objects/mock_apt"
	cfg.AppDefs["arryved-api"].Varz.Port = varzPort

	// handler will be the object under test
	cache := model.NewStatusCache()
	handler := http.HandlerFunc(NewConfiguredHandlerStatus(cfg, cache))

	// mock set of statuses and queue up for the handler
	statuses, _ := runners.GetStatuses(cfg)
	cache.SetStatuses(statuses)

	// make the request using a test handler + responser pair
	responder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/status", nil)
	handler.ServeHTTP(responder, req)

	// expect no err and an OK status
	assert.NoError(err)
	assert.Equal(200, responder.Code)

	// unmarshal to confirm confirm results
	results := map[string]model.Status{}
	err = json.Unmarshal(responder.Body.Bytes(), &results)

	// should have 11 results (from the mock config)
	assert.NoError(err)
	assert.Len(results, 11)

	// deeply check a specific result
	result := results["arryved-api"]
	assert.Equal(2, result.Versions.Installed.Major)
	assert.Equal(14, result.Versions.Installed.Minor)
	assert.Equal(2, result.Versions.Installed.Patch)
	assert.Equal(-1, result.Versions.Installed.Build)

	// deeply check the same result's health check result(s)
	assert.NotNil(result.Health)
	assert.Greater(len(result.Health), 0)
	assert.False(result.Health[0].Healthy)
	assert.False(result.Health[0].Unknown)
	assert.Equal(10010, result.Health[0].Port)

	// check the same result's health check result(s)
	assert.NotNil(result.Health)
	assert.Greater(len(result.Health), 0)
	assert.False(result.Health[0].Healthy)
	assert.False(result.Health[0].Unknown)
	assert.Equal(10010, result.Health[0].Port)

	// check the same result's running version
	assert.Equal(1, result.Versions.Running.Major)
	assert.Equal(13, result.Versions.Running.Minor)
	assert.Equal(0, result.Versions.Running.Patch)
	assert.Equal(-1, result.Versions.Running.Build)
}

func mockVarzListenerForStatus() int {
	mux := http.NewServeMux()
	mux.HandleFunc("/varz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
            "server.info": {
                "githash": "xxxxxxxxxx",
                "type": "API",
                "version": "1.13.0"
            }
        }`))
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.Start()

	time.Sleep(1 * time.Second)
	return srv.Listener.Addr().(*net.TCPAddr).Port
}
