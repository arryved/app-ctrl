package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	artifacts "cloud.google.com/go/artifactregistry/apiv1"
	artifactspb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/api/runners"
	"github.com/arryved/app-ctrl/daemon/model"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func ConfiguredHandlerStatus(cfg *config.Config, gceCache *runners.GCECache) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		httpStatus := http.StatusOK
		log.Infof("%s %s %s %d", r.RemoteAddr, r.Method, r.URL, httpStatus)

		urlElements := strings.Split(r.URL.String(), "/")

		if len(urlElements) != 6 {
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
		region := urlElements[4]
		variant := urlElements[5]

		if app == "any" || region == "any" || variant == "any" {
			clusterStatuses := make([]*ClusterStatus, 0)
			for i := range cfg.Topology[env].Clusters {
				clusterId := cfg.Topology[env].Clusters[i].Id
				if (app == clusterId.App || app == "any") &&
					(region == clusterId.Region || region == "any") &&
					(variant == clusterId.Variant || variant == "any") {
					clusterStatus, err := GetClusterStatus(cfg, gceCache, env, clusterId)
					if err != nil {
						log.Errorf("error fetching statuses: %v", err.Error())
						handleInternalServerError(w, err)
						return
					}
					clusterStatuses = append(clusterStatuses, clusterStatus)
				}
			}
			responseBody, err := json.Marshal(clusterStatuses)
			if err != nil {
				log.Errorf("error marshalling statuses: %v", err.Error())
				handleInternalServerError(w, err)
				return
			}

			log.Debugf("response body=%s", string(responseBody))
			w.WriteHeader(httpStatus)
			w.Write(responseBody)
			return
		}

		clusterStatus, err := GetClusterStatus(cfg, gceCache, env, config.ClusterId{
			App:     app,
			Region:  region,
			Variant: variant,
		})
		if err != nil {
			log.Errorf("error fetching statuses: %v", err.Error())
			handleInternalServerError(w, err)
			return
		}

		responseBody, err := json.Marshal(clusterStatus)
		if err != nil {
			log.Errorf("error marshalling statuses: %v", err.Error())
			handleInternalServerError(w, err)
			return
		}

		log.Debugf("response body=%s", string(responseBody))
		w.WriteHeader(httpStatus)
		w.Write(responseBody)
	}
}

type ClusterAttributes struct {
	Canaries []string `json:"canaries"`
}

type ClusterStatus struct {
	Id           *config.ClusterId        `json:"id"`
	HostStatuses map[string]*model.Status `json:"hostStatuses"`
	Attributes   *ClusterAttributes       `json:"attributes"`
}

type HostStatus map[string]*model.Status

func GetClusterStatus(cfg *config.Config, gceCache *runners.GCECache, env string, clusterId config.ClusterId) (*ClusterStatus, error) {
	// find the cluster in topology by id
	app := clusterId.App
	region := clusterId.Region
	variant := clusterId.Variant

	cluster, err := findClusterById(cfg, gceCache, env, config.ClusterId{App: app, Region: region, Variant: variant})
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		msg := fmt.Sprintf("no cluster found for app=%s in env=%s, region=%s, variant=%s", app, env, region, variant)
		log.Infof(msg)
		return nil, errors.New(msg)
	}
	if cluster.Runtime == "GKE" {
		return GetClusterStatusGKE(env, cfg, cluster)
	} else if cluster.Runtime == "GCE" {
		return GetClusterStatusGCE(env, cfg, cluster)
	}
	return nil, errors.New(fmt.Sprintf("unsupported cluster runtime %s", cluster.Runtime))
}

func GetClusterStatusGKE(env string, cfg *config.Config, cluster *config.Cluster) (*ClusterStatus, error) {
	k8sClient, err := createK8sClient(cfg.KubeConfigPath)
	if err != nil {
		return nil, err
	}

	podsClient := k8sClient.CoreV1().Pods("")
	pods, err := podsClient.List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", cluster.Id.App),
	})
	if err != nil {
		return nil, err
	}
	log.Debugf("%d pods found", len(pods.Items))

	clusterStatus := ClusterStatus{
		Id:           &cluster.Id,
		HostStatuses: make(map[string]*model.Status),
		Attributes: &ClusterAttributes{
			Canaries: []string{},
		},
	}

	for i := 0; i < len(pods.Items); i++ {
		// NOTE: initial convention is one container per pod; that's how deployables are assumed to be structured
		appName := cluster.Id.App
		podName := pods.Items[i].Name
		image := pods.Items[i].Status.ContainerStatuses[0].Image
		ready := pods.Items[i].Status.ContainerStatuses[0].Ready
		portList := pods.Items[i].Spec.Containers[0].Ports
		var ports []int32
		for _, port := range portList {
			ports = append(ports, port.ContainerPort)
		}
		version := getImageVersion(image)
		phase := pods.Items[i].Status.Phase

		log.Infof("app=%s pod=%s, version=%s, ready=%t, phase=%s, ports=%v", appName, podName, version, ready, phase, ports)
		healthResults := make([]model.HealthResult, 0)
		for j := 0; j < len(ports); j++ {
			healthResults = append(healthResults, model.HealthResult{
				Port:    int(ports[j]),
				Healthy: ready,
				OOR:     false, // in GKE this is handled by the service object
				Unknown: false,
			})
		}
		parsedVersion, err := model.ParseVersion(version)
		if err != nil {
			log.Warnf("when getting cluster status, could not parse version string %s: %s", version, err.Error())
			parsedVersion = model.Version{
				Major: -1,
				Minor: -1,
				Patch: -1,
				Build: -1,
			}
		}
		status := model.Status{
			Versions: model.Versions{
				Config:    0,
				Installed: &parsedVersion,
				Running:   &parsedVersion,
			},
			Health: healthResults,
		}
		clusterStatus.HostStatuses[podName] = &status
	}

	log.Infof("clusterStatus=%v", clusterStatus)
	return &clusterStatus, nil
}

func GetClusterStatusGCE(env string, cfg *config.Config, cluster *config.Cluster) (*ClusterStatus, error) {
	hosts := cluster.Hosts
	app := cluster.Id.App
	log.Debugf("%d hosts found for env=%s, app=%s", len(hosts), env, app)

	clusterStatus := ClusterStatus{
		Id:           &cluster.Id,
		HostStatuses: make(map[string]*model.Status),
		Attributes: &ClusterAttributes{
			Canaries: []string{},
		},
	}

	ch := make(chan map[string]*model.Status)
	for name, _ := range hosts {
		go func(ch chan map[string]*model.Status, name string) {
			hostStatus, err := GetHostStatus(
				cfg.AppControlDScheme, name, cfg.AppControlDPort, cfg.AppControlDPSKPath, cfg.ReadTimeoutS)
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
			clusterStatus.HostStatuses[name] = status
			if hosts[name].Canary {
				clusterStatus.Attributes.Canaries = append(clusterStatus.Attributes.Canaries, name)
			}
		}
	}

	return &clusterStatus, nil
}

func GetHostStatus(scheme string, host string, port int, pskPath string, timeoutS int) (*HostStatus, error) {
	url := fmt.Sprintf("%s://%s:%d/status", scheme, host, port)
	tr := http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   time.Duration(timeoutS) * time.Second,
		Transport: &tr,
	}

	req, err := http.NewRequest("GET", url, nil)
	psk := fmt.Sprintf("Bearer %s", readPSKFromPath(pskPath))
	req.Header.Set("Authorization", psk)
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

func readPSKFromPath(pskPath string) string {
	pskFromFile, err := ioutil.ReadFile(pskPath)
	if err != nil {
		log.Warnf("couldn't read PSK from path=%s", pskPath)
		return ""
	}
	return strings.TrimSpace(string(pskFromFile))
}

func getImageVersion(image string) string {
	version := "unknown"
	splitURL := strings.Split(image, "/")
	lastElement := splitURL[len(splitURL)-1]
	// split the last element by ":"
	splitVersion := strings.Split(lastElement, ":")
	// if there is only one element after splitting, assume latest
	if len(splitVersion) == 1 {
		version = "latest"
	} else {
		// otherwise, get the last element after splitting
		version = splitVersion[len(splitVersion)-1]
	}

	// if the version is "latest", use the repo url to find all tags on the "latest" version.
	if version == "latest" {
		ctx := context.Background()
		artifactsClient, err := createArtifactsClient(ctx)
		if err != nil {
			log.Warnf("could get an artifacts client - %s", err)
			return "latest"
		}
		defer artifactsClient.Close()
		version, err = fetchGKEImageVersion(ctx, artifactsClient, image)
		if err != nil {
			log.Warnf("could not get a version for :latest via the artifacts client - %s", err)
			return "latest"
		}
	}
	// if the "latest" tag isn't present, find the image with the most recent timestamp.
	// if there is no version tag on it, return the image hash as a fallback
	return version
}

func fetchGKEImageVersion(ctx context.Context, client *artifacts.Client, image string) (string, error) {
	// try go get version from :latest in the repo based on image uri
	// e.g. uri: us-central1-docker.pkg.dev/arryved-tools/product-docker/poserp-app
	locationFqdn := strings.Split(image, "/")[0]
	location := strings.Replace(locationFqdn, "-docker.pkg.dev", "", -1)
	splitString := strings.Split(image, "/")
	project := splitString[1]
	repo := splitString[2]
	app := strings.Split(splitString[3], ":")[0]

	parent := fmt.Sprintf("projects/%s/locations/%s/repositories/%s/packages/%s", project, location, repo, app)
	response := client.ListTags(ctx, &artifactspb.ListTagsRequest{Parent: parent})
	log.Debugf("parent=%s, response=%v", parent, response)

	tagsByHash := make(map[string][]string)
	latestHash := ""
	for {
		r, err := response.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "unknown", err
		}
		name := r.Name[strings.LastIndex(r.Name, "/")+1:]
		hash := r.Version[strings.LastIndex(r.Version, "/")+1:]
		log.Debugf("name=%s, hash=%s", name, hash)
		if name == "latest" {
			latestHash = hash
		}
		tagsByHash[hash] = append(tagsByHash[hash], name)
	}
	if latestHash == "" {
		return "unknown", nil
	} else {
		tags := onlyVersionTags(tagsByHash[latestHash])
		if len(tags) > 2 {
			// this isn't supposed to happen
			return "unknown", errors.New(fmt.Sprintf("latest has multiple tripartite version tags %v", tags))
		}
		for _, tag := range tags {
			if tag != "latest" {
				log.Debugf("latest for %s appears to be %s", image, tag)
				return tag, nil
			}
		}
	}
	return "unknown", nil
}

func onlyVersionTags(tags []string) []string {
	// Regular expression to match x.y.z scheme
	re := regexp.MustCompile(`^\d+\.\d+\.\d+$`)

	filteredTags := make([]string, 0)
	for _, tag := range tags {
		if re.MatchString(tag) {
			filteredTags = append(filteredTags, tag)
		}
	}
	return filteredTags
}

func createArtifactsClient(ctx context.Context) (*artifacts.Client, error) {
	c, err := artifacts.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func createK8sClient(kubeconfigPath string) (*kubernetes.Clientset, error) {
	// NOTE: This is the out-of-cluster auth workflow; there is another
	// meant for in-cluster workflow
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Errorf("kubeconfig build failed - %s", err)
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}
