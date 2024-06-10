package api

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/api/queue"
)

type Api struct {
	cfg *config.Config
}

func (a *Api) Start() error {
	cfg := a.cfg

	queueClient, err := queue.NewClient(cfg.Queue)
	if err != nil {
		log.Errorf("could not get a queue client, error=%s", err.Error())
		return err
	}
	jobQueue := queue.NewQueue(cfg.Queue, queueClient)

	mux := http.NewServeMux()
	mux.HandleFunc("/status/", ConfiguredHandlerStatus(cfg))
	mux.HandleFunc("/deploy/", ConfiguredHandlerDeploy(cfg, jobQueue))

	tlsConfig := &tls.Config{
		CipherSuites:             CipherSuitesFromConfig(cfg.TLS.Ciphers),
		MinVersion:               TLSVersionFromConfig(cfg.TLS.MinVersion),
		PreferServerCipherSuites: true,
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.ReadTimeoutS) * time.Second,
		TLSConfig:    tlsConfig,
		WriteTimeout: time.Duration(cfg.WriteTimeoutS) * time.Second,
	}

	log.Infof("Starting HTTPS listener on port=%d", cfg.Port)
	err = srv.ListenAndServeTLS(
		cfg.CrtPath,
		cfg.KeyPath,
	)
	if err != nil {
		log.Errorf("Error seen when starting listener: %v", err)
	}

	log.Info("Finishing up")
	return err
}

func New(cfg *config.Config) *Api {
	api := &Api{cfg: cfg}
	return api
}

func CipherSuitesFromConfig(configuredCiphers []string) []uint16 {
	result := []uint16{}
	for _, cipherSuite := range tls.CipherSuites() {
		if contains(configuredCiphers, cipherSuite.Name) {
			result = append(result, cipherSuite.ID)
		}
	}
	return result
}

func TLSVersionFromConfig(versionString string) uint16 {
	version := tls.VersionTLS12

	switch versionString {
	case "1.0":
		return tls.VersionTLS10
	case "1.1":
		return tls.VersionTLS11
	case "1.2":
		return tls.VersionTLS12
	case "1.3":
		return tls.VersionTLS13
	}

	return uint16(version)
}

func contains(list []string, match string) bool {
	for _, element := range list {
		if element == match {
			return true
		}
	}
	return false
}

func handleBadRequest(w http.ResponseWriter, msg string) {
	httpStatus := http.StatusBadRequest
	errorBody := fmt.Sprintf("{\"error\": \"%s\"}", msg)
	w.WriteHeader(httpStatus)
	w.Write([]byte(errorBody))
	return
}

func handleNotFound(w http.ResponseWriter, msg string) {
	httpStatus := http.StatusNotFound
	errorBody := fmt.Sprintf("{\"not found\": \"%s\"}", msg)
	w.WriteHeader(httpStatus)
	w.Write([]byte(errorBody))
	return
}

func handleInternalServerError(w http.ResponseWriter, err error) {
	httpStatus := http.StatusInternalServerError
	errorBody := fmt.Sprintf("{\"error\": \"%s\"}", err.Error())
	w.WriteHeader(httpStatus)
	w.Write([]byte(errorBody))
	return
}

func handleMethodNotAllowed(w http.ResponseWriter, msg string) {
	httpStatus := http.StatusMethodNotAllowed
	errorBody := fmt.Sprintf("{\"error\": \"%s\"}", msg)
	w.WriteHeader(httpStatus)
	w.Write([]byte(errorBody))
	return
}

func handleForbidden(w http.ResponseWriter, msg string) {
	httpStatus := http.StatusForbidden
	errorBody := fmt.Sprintf("{\"error\": \"%s\"}", msg)
	w.WriteHeader(httpStatus)
	w.Write([]byte(errorBody))
	return
}

func handleUnauthorized(w http.ResponseWriter, msg string) {
	httpStatus := http.StatusUnauthorized
	errorBody := fmt.Sprintf("{\"error\": \"%s\"}", msg)
	w.WriteHeader(httpStatus)
	w.Write([]byte(errorBody))
	return
}

func extractQueryParams(r *http.Request) map[string]string {
	params := make(map[string]string)

	query := r.URL.Query()
	for key, value := range query {
		params[key] = value[0]
	}
	return params
}

func findClusterById(cfg *config.Config, env string, id config.ClusterId) (*config.Cluster, error) {
	for _, cluster := range cfg.Topology[env].Clusters {
		log.Debugf("cluster: %v", cluster)
		cid := cluster.Id
		if cid.App == id.App && cid.Region == id.Region && cid.Variant == id.Variant {
			return &cluster, nil
		}
	}
	return nil, nil
}
