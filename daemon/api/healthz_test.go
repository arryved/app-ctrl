//go:build !integration

package api

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
	"github.com/arryved/app-ctrl/daemon/runners"
)

func TestHealthzHandler(t *testing.T) {
	assert := assert.New(t)

	// mock config, change varz port to match mock listener
	cfg := config.Load("../config/mock-config.yml")
	srv, varzPort := mockVarzListenerForHealthz()
	defer srv.Close()

	cfg.AptPath = "./test_objects/mock_apt"
	cfg.AppDefs["arryved-merchant"].Varz.Port = varzPort
	cfg.AppDefs["arryved-merchant"].Healthz[0].Port = varzPort

	// stub channel to configure the handler with
	// handler will be the object under test
	cache := model.NewStatusCache()
	handler := http.HandlerFunc(NewConfiguredHandlerHealthz(cfg, cache))

	// mock set of statuses and queue up for the handler via the cache
	statuses, err := runners.GetStatuses(cfg)
	cache.SetStatuses(statuses)
	assert.NoError(err)

	// make the request using a test handler + responser pair
	responder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/healthz?app=arryved-merchant", nil)
	handler.ServeHTTP(responder, req)

	// expect no err and an OK status
	assert.NoError(err)
	assert.Equal(200, responder.Code)
	assert.Equal("OK", string(responder.Body.Bytes()))
}

func mockVarzListenerForHealthz() (*httptest.Server, int) {
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
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`OK`))
	})

	srv := httptest.NewServer(mux)
	return srv, srv.Listener.Addr().(*net.TCPAddr).Port
}
