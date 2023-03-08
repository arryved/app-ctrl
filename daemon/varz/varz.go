package varz

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/arryved/app-ctrl/daemon/config"
)

type VarzResult struct {
	ServerInfo ServerInfo `json:"server.info"`
}

type ServerInfo struct {
	// version
	Version string `json:"version"`

	// commit hash
	GitHash string `json:"githash"`

	// type is basically an app variant
	Type string `json:"type"`
}

// check a varz port
func Check(varzSpec config.Varz) VarzResult {
	port := varzSpec.Port
	useTLS := varzSpec.TLS
	result := VarzResult{}

	scheme := "http"
	if useTLS {
		scheme = "https"
	}

	// NOTE: This skips TLS verification because of the state of our health check TLS setup
	// ca. 2023 Q1, which relies on hard-coded certificates that have long since expired.
	// not *that* big a deal for localhost checks
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: transport}

	url := fmt.Sprintf("%s://localhost:%d/varz", scheme, port)
	resp, err := client.Get(url)
	if err != nil {
		log.Warnf("Could not retrieve varz from url=%s, error=%v", url, err)
		return result
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Warnf("Could not read varz body from url=%s, error=%v", url, err)
		return result
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Warnf("Could not unmarshale varz body from url=%s, error=%v", url, err)
		return result
	}

	return result
}
