package runners

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
	apiconfig "github.com/arryved/app-ctrl/api/config"
)

type AppControlMeta struct {
	Clusters []apiconfig.ClusterId `json:"clusters"`
}

func getMetadata(path string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/"+path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func GetAppControlMetadata(app string) (string, *apiconfig.ClusterId, error) {
	projectIdBytes, err := getMetadata("project/project-id")
	if err != nil {
		log.Errorf("metadata fetch failed for project/project-id err=%s", err.Error())
		return "", nil, err
	}
	instanceNameBytes, err := getMetadata("instance/name")
	if err != nil {
		log.Errorf("metadata fetch failed for instance/name err=%s", err.Error())
		return "", nil, err
	}

	projectId := string(projectIdBytes)
	instanceName := string(instanceNameBytes)
	env := envFromProjectIdAndHost(projectId, instanceName)

	metaJson, err := getMetadata("instance/attributes/app-control")
	if err != nil {
		log.Errorf("metadata fetch failed for instance/attributs/app-control err=%s", err.Error())
		return "", nil, err
	}

	var appControl AppControlMeta
	err = json.Unmarshal([]byte(metaJson), &appControl)
	if err != nil {
		log.Errorf("metadata unmarshal failed on metadata value=%s err=%s", string(metaJson), err.Error())
		return "", nil, err
	}
	for _, clusterId := range appControl.Clusters {
		if clusterId.App == app {
			return env, &clusterId, nil
		}
	}

	// this really shouldn't happen, so flag it as an error
	msg := "Could not find app in host metadata under app-control key"
	log.Error(msg)
	return "", nil, fmt.Errorf(msg)
}

// temporary until there's a canon library/api
func envFromProjectIdAndHost(projectId, instanceName string) string {
	envByProjectId := map[string]string{
		"arryved-177921":  "dev",
		"arryved-staging": "stg",
		"arryved-prod":    "prod",
		"arryved-secure":  "cde",
	}
	env, ok := envByProjectId[projectId]
	if !ok {
		log.Warnf("could not determine env from projectId=%s", projectId)
		return ""
	}

	// remove this hack once sandbox is gone or moved its own project env
	if env == "dev" && instanceName == "sandbox-api-container" {
		return "sandbox"
	}
	return env
}
