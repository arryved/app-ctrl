package api

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	compute "google.golang.org/api/compute/v1"

	"github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/api/queue"
	"github.com/arryved/app-ctrl/api/runners"
)

// TODO replace with API or canon lookup or fix tools/internal and sandbox/dev incongruities
var fqdnMap = map[string]string{
	"cde":     "cde.arryved.com",
	"dev":     "dev.arryved.com",
	"dev-int": "dev-int.arryved.com",
	"prod":    "prod.arryved.com",
	"sandbox": "dev.arryved.com",
	"stg":     "stg.arryved.com",
	"tools":   "internal.arryved.com",
}

type Api struct {
	cfg      *config.Config
	gceCache *runners.GCECache
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
	mux.HandleFunc("/status/", ConfiguredHandlerStatus(cfg, a.gceCache))
	mux.HandleFunc("/deploy/", ConfiguredHandlerDeploy(cfg, a.gceCache, jobQueue))
	mux.HandleFunc("/secrets/", ConfiguredHandlerSecrets(cfg))

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

func New(cfg *config.Config, cache *runners.GCECache) *Api {
	api := &Api{
		cfg:      cfg,
		gceCache: cache,
	}
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

func handleConflict(w http.ResponseWriter, msg string) {
	httpStatus := http.StatusConflict
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

func findClusterById(cfg *config.Config, gceCache *runners.GCECache, env string, id config.ClusterId) (*config.Cluster, error) {
	app := id.App
	region := id.Region
	variant := id.Variant
	log.Infof("searching for target ID app=%s, region=%s, variant=%s", app, region, variant)
	for _, cluster := range cfg.Topology[env].Clusters {
		configId := cluster.Id
		if configId.App == app && configId.Region == region && configId.Variant == variant {
			log.Infof("found target ID app=%s, region=%s, variant=%s", app, region, variant)
			// if GCE clusters if host list is empty, use discovery cache
			if cluster.Runtime == "GCE" && len(cluster.Hosts) == 0 {
				// use cache to get hosts
				instances := gceCache.Get()[configId]
				// find & set host values, canary markers
				cluster.Hosts = instancesToHostList(instances, env)
			}
			return &cluster, nil
		}
	}
	return nil, nil
}

func instancesToHostList(instances []*compute.Instance, env string) map[string]config.Host {
	hosts := map[string]config.Host{}
	for _, instance := range instances {
		labels := instance.Labels
		canary := false
		canaryStr, ok := labels["canary"]
		if ok {
			if canaryStr == "true" {
				canary = true
			}
		}
		fqdn := fmt.Sprintf("%s.%s", instance.Name, fqdnMap[env])
		hosts[fqdn] = config.Host{
			Canary: canary,
		}
	}
	return hosts
}
