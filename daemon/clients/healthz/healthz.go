package healthz

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

// check a healthz port
func Check(healthzSpec config.Healthz) model.HealthResult {
	port := healthzSpec.Port
	useTLS := healthzSpec.TLS
	result := model.HealthResult{Port: port}

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

	url := fmt.Sprintf("%s://localhost:%d/healthz", scheme, port)
	resp, err := client.Get(url)
	if err != nil {
		log.Debugf("url=%s error = %v", url, err)
		if strings.Contains(err.Error(), "HTTP response to HTTPS client") {
			result.Unknown = true
			return result
		}
		result.Healthy = false
		result.Unknown = false
		return result
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		result.Unknown = true
		return result
	}

	result.Healthy = strings.TrimRight(string(body), "\n") == "OK"
	result.Unknown = false
	return result
}
