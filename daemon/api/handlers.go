package api

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

func ConfiguredHandlerStatus(cfg *config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		statuses, _ := getStatuses(cfg)
		// TODO ^^ handle error
		w.Header().Set("content-type", "application/json")
		responseBody, _ := json.Marshal(statuses)
		// TODO ^^ handle error
		w.Write(responseBody)
	}
}

func registerHandlers(mux *http.ServeMux, cfg *config.Config) {
	mux.HandleFunc("/status", ConfiguredHandlerStatus(cfg))
}

func getStatuses(cfg *config.Config) (map[string]model.Status, error) {
	statuses := map[string]model.Status{}

	versionsByApp, err := getInstalledVersions(cfg)
	if err != nil {
		return statuses, err
	}

	for app, version := range versionsByApp {
		status := model.Status{
			Versions: model.Versions{
				Installed: &version,
			},
		}
		statuses[app] = status
	}
	return statuses, nil
}

func getInstalledVersions(cfg *config.Config) (map[string]model.Version, error) {
	statuses := map[string]model.Version{}

	args := "list --installed"
	cmd := exec.Command(cfg.AptPath, strings.Split(args, " ")...)

	stdout, _ := cmd.StdoutPipe()
	cmd.Start()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()

		// ignore non-content lines
		if !strings.Contains(line, "[installed]") {
			continue
		}

		// parse line, format: "arryved-api/unknown,now 2.14.2 amd64 [installed]"
		fields := strings.Split(line, "/")
		name := fields[0]

		// skip apps that aren't in the AppDefs
		_, ok := cfg.AppDefs[name]
		if !ok {
			continue
		}

		// parse out version
		fields = strings.Split(fields[1], " ")
		installedVersion, err := model.ParseVersion(fields[1])
		if err != nil {
			log.Warnf("version %s could not be parsed: %v", fields[1], err)
			return statuses, err
		}

		// select based on known apps
		statuses[name] = installedVersion
	}

	cmd.Wait()
	return statuses, nil
}
