//go:build !integration

package api

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/arryved/app-ctrl/api/config"
)

func mockAppControlD() int {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		data, _ := os.ReadFile("./test_objects/status-prod-arryved-api.json")
		w.Write(data)
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.Start()

	time.Sleep(1 * time.Second)
	return srv.Listener.Addr().(*net.TCPAddr).Port
}

func TestGetHostStatus(t *testing.T) {
	assert := assert.New(t)
	host := "localhost"
	port := mockAppControlD()

	status, err := GetHostStatus("http", host, port, 2)

	// check basic attributes of response
	assert.Nil(err)
	assert.NotNil(status)
	assert.Equal(5, len(*status))

	// check deep example
	assert.Equal(2, (*status)["arryved-api"].Versions.Installed.Major)
	assert.Equal(14, (*status)["arryved-api"].Versions.Installed.Minor)
	assert.Equal(2, (*status)["arryved-api"].Versions.Installed.Patch)
	assert.Equal(-1, (*status)["arryved-api"].Versions.Installed.Build)
}

func TestGetClusterStatuses(t *testing.T) {
	assert := assert.New(t)
	port := mockAppControlD()
	cfg := config.Load("")
	cfg.AppControlDPort = port
	cfg.AppControlDScheme = "http"
	cfg.Topology = map[string]config.Environment{
		"dev": config.Environment{
			Clusters: map[string]config.Cluster{
				"arryved-api": config.Cluster{
					Hosts: map[string]config.Host{
						"localhost": config.Host{},
						"127.0.0.1": config.Host{},
					},
				},
			},
		},
	}

	status, err := GetClusterStatus(cfg, "dev", "arryved-api")

	assert.Nil(err)
	assert.NotNil(status)
	assert.Len(*status, 2)
	assert.Contains(*status, "localhost")
	assert.Contains(*status, "127.0.0.1")

	assert.Equal(2, (*((*status)["localhost"])).Versions.Installed.Major)
	assert.Equal(2, (*((*status)["127.0.0.1"])).Versions.Installed.Major)
}
