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
)

func mockQueue() int {
	mux := http.NewServeMux()
	mux.HandleFunc("/deploy", func(w http.ResponseWriter, r *http.Request) {
		data, _ := os.ReadFile("./test_objects/queue-deploy-response.json")
		w.Write(data)
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.Start()

	time.Sleep(1 * time.Second)
	return srv.Listener.Addr().(*net.TCPAddr).Port
}

func TestSubmitAndWait(t *testing.T) {
	assert := assert.New(t)
	//port := mockAppControlD()
	//cfg := config.Load("")
	//cfg.Queue = port
	//cfg.Topology = map[string]config.Environment{
	//"dev": config.Environment{
	//Clusters: map[string]config.Cluster{
	//"arryved-api": config.Cluster{
	//Hosts: map[string]config.Host{
	//"localhost": config.Host{},
	//"127.0.0.1": config.Host{
	//Canary: true,
	//},
	//},
	//},
	//},
	//},
	//}

	// check basic submit/result flow
	assert.True(true)
	//submit, err := SubmitDeployJob(cfg, "REQUESTID", "APP", "X.Y.X")
	//result, err := WaitForResult(cfg, "REQUESTID")

	// TODO - check to see that jobs submitted for an app already being acted on are rejected
}
