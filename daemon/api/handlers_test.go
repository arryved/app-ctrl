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
)

func mockVarzListener() int {
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

func TestStatusHandler(t *testing.T) {
	assert := assert.New(t)

	// mock config, change varz port to match mock listener
	cfg := config.Load("../config/mock-config.yml")
	cfg.AptPath = "./mock_apt"
	varzPort := mockVarzListener()
	cfg.AppDefs["arryved-api"].Varz.Port = varzPort

	// stub channel and cache to configure the handler with
	// handler will be the object under test
	ch := make(chan map[string]model.Status, 1)
	cache := make(map[string]model.Status)
	handler := http.HandlerFunc(ConfiguredHandlerStatus(cfg, ch, cache))

	// get one status and queue up for the handler via the channel
	statuses, _ := getStatuses(cfg)
	ch <- statuses

	// make the request using a test handler + responser pair
	responder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/status", nil)
	handler.ServeHTTP(responder, req)

	// expect no err and an OK status
	assert.Nil(err)
	assert.Equal(200, responder.Code)

	// unmarshal to confirm confirm results
	results := map[string]model.Status{}
	err = json.Unmarshal(responder.Body.Bytes(), &results)

	// should have 11 results (from the mock config)
	assert.Nil(err)
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

	// deeply check the same result's running version
	assert.Equal(1, result.Versions.Running.Major)
	assert.Equal(13, result.Versions.Running.Minor)
	assert.Equal(0, result.Versions.Running.Patch)
	assert.Equal(-1, result.Versions.Running.Build)
}
