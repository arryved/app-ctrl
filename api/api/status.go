package api

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func ConfiguredHandlerStatus(cfg *config.Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		httpStatus := http.StatusOK
		log.Infof("%s %s %s %d", r.RemoteAddr, r.Method, r.URL, httpStatus)

		urlElements := strings.Split(r.URL.String(), "/")
		if len(urlElements) != 4 {
			httpStatus := http.StatusNotFound
			msg := fmt.Sprintf("invalid request path: %s", r.URL)
			log.Infof(msg)
			errorBody := fmt.Sprintf("{\"error\": \"%s\"}", msg)
			w.WriteHeader(httpStatus)
			w.Write([]byte(errorBody))
			return
		}

		env := urlElements[2]
		app := urlElements[3]

		clusterStatus, err := GetClusterStatus(cfg, env, app)
		if err != nil {
			httpStatus = http.StatusInternalServerError
			errorBody := fmt.Sprintf("{\"error\": \"%s\"}", err.Error())
			w.WriteHeader(httpStatus)
			w.Write([]byte(errorBody))
			return
		}

		responseBody, err := json.Marshal(clusterStatus)
		if err != nil {
			httpStatus = http.StatusInternalServerError
			log.Errorf("error marshalling statuses: %v", err.Error())
			errorBody := fmt.Sprintf("{\"error\": \"%s\"}", err.Error())
			w.WriteHeader(httpStatus)
			w.Write([]byte(errorBody))
			return
		}

		log.Debugf("response body=%s", string(responseBody))
		w.WriteHeader(httpStatus)
		w.Write(responseBody)
	}
}

type ClusterStatus map[string]*model.Status
type HostStatus map[string]*model.Status

func GetClusterStatus(cfg *config.Config, env string, app string) (*ClusterStatus, error) {
	log.Infof("looking up env=%s, app=%s", env, app)

	_, ok := cfg.Topology[env]
	if !ok {
		msg := fmt.Sprintf("no such env=%s in topology", env)
		log.Infof(msg)
		return nil, errors.New(msg)
	}

	_, ok = cfg.Topology[env].Clusters[app]
	if !ok {
		msg := fmt.Sprintf("no cluster for app=%s in env=%s", app, env)
		log.Infof(msg)
		return nil, errors.New(msg)
	}

	hosts := cfg.Topology[env].Clusters[app].Hosts
	log.Infof("%d hosts found for env=%s, app=%s", len(hosts), env, app)

	clusterStatus := make(ClusterStatus)
	ch := make(chan map[string]*model.Status)
	for name, _ := range hosts {
		go func(ch chan map[string]*model.Status, name string) {
			hostStatus, err := GetHostStatus(
				cfg.AppControlDScheme, name, cfg.AppControlDPort, cfg.ReadTimeoutS)
			result := make(map[string]*model.Status)
			if err != nil {
				log.Warnf("no status retrieved for host=%s", name)
				result[name] = nil
			} else {
				log.Debugf("status retrieved for host=%s", name)
				result[name] = (*hostStatus)[app]
			}
			ch <- result
		}(ch, name)
	}

	for _, _ = range hosts {
		result := <-ch
		for name, status := range result {
			clusterStatus[name] = status
		}
	}

	return &clusterStatus, nil
}

func GetHostStatus(scheme string, host string, port int, timeoutS int) (*HostStatus, error) {
	url := fmt.Sprintf("%s://%s:%d/status", scheme, host, port)
	tr := http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   time.Duration(timeoutS) * time.Second,
		Transport: &tr,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Warnf("Failed to create /status request to app-controld on host=%s, err=%v", host, err)
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Warnf("Failed to execute /status request to app-controld on host=%s, err=%v", host, err)
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Warnf("Failed body read on /status request to app-controld on host=%s, err=%v", host, err)
		return nil, err
	}

	status := make(HostStatus)
	err = json.Unmarshal(body, &status)
	if err != nil {
		log.Warnf("Failed to unmarshal response from app-controld on host=%s, err=%v", host, err)
		return nil, err
	}

	return &status, err
}
