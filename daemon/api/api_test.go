//go:build !integration

package api

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

// check that New returns an API object with the provided config
func TestNew(t *testing.T) {
	assert := assert.New(t)

	cfg := config.Load("../config/mock-config.yml")
	ch := make(chan map[string]model.Status, 1)
	api := New(cfg, ch)

	assert.Equal(cfg, api.cfg)
}

// check that start spins up an HTTPS server
func TestStart(t *testing.T) {
	assert := assert.New(t)
	cfg := config.Load("../config/mock-config.yml")
	cwd, _ := os.Getwd()
	varDir := path.Join(cwd, "../var")
	cfg.CrtPath = path.Join(varDir, "service.crt")
	cfg.KeyPath = path.Join(varDir, "service.key")
	ch := make(chan map[string]model.Status, 1)

	api := New(cfg, ch)
	go api.Start()
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	resp, err := http.Get(fmt.Sprintf("https://localhost:%d/", cfg.Port))

	assert.Nil(err)
	assert.NotNil(resp)
	assert.Equal("404 Not Found", resp.Status)
	assert.Equal(404, resp.StatusCode)
}
