//go:build !integration

package varz

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/arryved/app-ctrl/daemon/config"
)

func mockVarzListener() int {
	mux := http.NewServeMux()
	mux.HandleFunc("/varz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"server.info": {
				"date": "20230224.2018",
				"githash": "69117d3b8d",
				"tagname": "",
				"compiled": "1677295089000",
				"type": "API",
				"version": "2.14.2",
				"branch": "r-backend-2.14"
			}
			}`))
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.Start()

	time.Sleep(1 * time.Second)
	return srv.Listener.Addr().(*net.TCPAddr).Port
}

// verify varz checking works
func TestCheck(t *testing.T) {
	assert := assert.New(t)
	port := mockVarzListener()
	varzSpec := config.Varz{Port: port}

	result := Check(varzSpec)

	assert.Equal("2.14.2", result.ServerInfo.Version)
	assert.Equal("69117d3b8d", result.ServerInfo.GitHash)
	assert.Equal("API", result.ServerInfo.Type)
}
