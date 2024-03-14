package runners

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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
                "githash": "xxxxxxxxxx",
                "type": "API",
                "version": "1.13.0"
            }
        }`))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`OK`))
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.Start()

	time.Sleep(1 * time.Second)
	return srv.Listener.Addr().(*net.TCPAddr).Port
}

func TestGetStatuses(t *testing.T) {
	assert := assert.New(t)

	// mock app root
	tempDir, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tempDir)

	// mock config, change varz port to match mock listener
	varzPort := mockVarzListener()
	cfg := config.Load("../config/mock-config.yml")
	cfg.AptPath = "../api/test_objects/mock_apt"
	cfg.AppDefs["arryved-merchant"] = config.AppDef{
		Varz: &config.Varz{
			Port: varzPort,
		},
		Healthz: []config.Healthz{
			config.Healthz{
				Port: varzPort,
			},
		},
		AppRoot: tempDir, // set app root to be at tempdir for testing OOR
		Type:    config.Online,
	}

	statuses, err := GetStatuses(cfg)

	assert.Nil(err)
	assert.Len(statuses, 11)
	assert.True(statuses["arryved-merchant"].Health[0].Healthy)
	assert.False(statuses["arryved-merchant"].Health[0].OOR)

	// take arryved-merchant out of rotation (OOR) and try again
	err = SetOOR(cfg.AppDefs["arryved-merchant"])
	assert.Nil(err)
	statuses, err = GetStatuses(cfg)

	assert.Nil(err)
	assert.Len(statuses, 11)
	// with OOR set, this should now be false
	assert.False(statuses["arryved-merchant"].Health[0].Healthy)
	assert.True(statuses["arryved-merchant"].Health[0].OOR)
}
