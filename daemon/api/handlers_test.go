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
	varzPort := mockVarzListener()

	cfg := config.Load("../config/mock-config.yml")
	cfg.AptPath = "./mock_apt"
	cfg.AppDefs["arryved-api"].Varz.Port = varzPort

	responder := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfiguredHandlerStatus(cfg))
	req, err := http.NewRequest("GET", "/status", nil)

	handler.ServeHTTP(responder, req)

	assert.Nil(err)
	assert.Equal(200, responder.Code)

	// unmarshal and confirm results
	results := map[string]model.Status{}
	err = json.Unmarshal(responder.Body.Bytes(), &results)

	assert.Nil(err)
	assert.Len(results, 11)

	// check a specific result
	result := results["arryved-api"]
	assert.Equal(2, result.Versions.Installed.Major)
	assert.Equal(14, result.Versions.Installed.Minor)
	assert.Equal(2, result.Versions.Installed.Patch)
	assert.Equal(-1, result.Versions.Installed.Build)

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
