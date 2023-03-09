//go:build !integration

package healthz

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/arryved/app-ctrl/daemon/config"
)

func mockHealthzListener(response string) int {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.Start()

	time.Sleep(1 * time.Second)
	return srv.Listener.Addr().(*net.TCPAddr).Port
}

// verify healthz checking works
func TestCheck(t *testing.T) {
	assert := assert.New(t)
	port := mockHealthzListener("OK")
	healthzSpec := config.Healthz{Port: port}

	result := Check(healthzSpec)

	assert.Equal(port, result.Port)
	assert.False(result.Unknown)
	assert.True(result.Healthy)
}

// TLS check on non-TLS port
func TestCheckTLSUnknown(t *testing.T) {
	assert := assert.New(t)
	port := mockHealthzListener("OK")
	healthzSpec := config.Healthz{Port: port, TLS: true}

	result := Check(healthzSpec)

	assert.Equal(port, result.Port)
	assert.True(result.Unknown)
}

// Check up port w/ non-OK status
func TestCheckFail(t *testing.T) {
	assert := assert.New(t)
	port := mockHealthzListener("ERROR")
	healthzSpec := config.Healthz{Port: port}

	result := Check(healthzSpec)

	assert.Equal(port, result.Port)
	assert.False(result.Unknown)
	assert.False(result.Healthy)
}

// Check on down port
func TestCheckDown(t *testing.T) {
	assert := assert.New(t)
	port := mockHealthzListener("ERROR")
	healthzSpec := config.Healthz{Port: port - 1}

	result := Check(healthzSpec)

	assert.Equal(port-1, result.Port)
	assert.False(result.Unknown)
	assert.False(result.Healthy)
}
