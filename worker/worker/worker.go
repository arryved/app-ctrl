package worker

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iterator"
	yaml "gopkg.in/yaml.v3"

	apiconfig "github.com/arryved/app-ctrl/api/config"
	productconfig "github.com/arryved/app-ctrl/api/config/product"
	"github.com/arryved/app-ctrl/api/queue"
	"github.com/arryved/app-ctrl/worker/config"
	"github.com/arryved/app-ctrl/worker/gce"
	"github.com/arryved/app-ctrl/worker/gke"
	appcontrold "github.com/arryved/app-ctrl/daemon/api"
)

type Worker struct {
	cfg     *config.Config
	queue   *queue.Queue
	compute *gce.Client
}

type JobResult struct {
	ActionStatus  string
	ClusterStatus string
	Detail        string
}

func (w *Worker) Start() {
	var wg sync.WaitGroup
	for i := 0; i < w.cfg.MaxJobThreads; i++ {
		log.Infof("spin up thread %d...", i)
		wg.Add(1)
		go func() {
			for {
				log.Debugf("checking queue...")
				job, err := w.queue.Dequeue()
				if err != nil {
					log.Errorf("dequeue error=%s, sleeping...", err.Error())
					time.Sleep(5 * time.Second)
					continue
				}
				if job.Id == "" {
					log.Debugf("dequeue timeout, sleeping...")
					time.Sleep(5 * time.Second)
					continue
				}
				log.Infof("job dequeued job=%v", job)
				result, err := w.ProcessJob(job)
				log.Infof("job finished result=%v", result)
				if err != nil {
					log.Errorf("job error=%s", err.Error())
				}
				log.Infof("thread sleep...")
				time.Sleep(5 * time.Second)
			}
		}()
	}
	wg.Wait()
}

func (w *Worker) ProcessJob(job *queue.Job) (*JobResult, error) {
	switch job.Action {
	case "DEPLOY":
		msg := fmt.Sprintf("%s action detected for job id=%s", job.Action, job.Id)
		log.Infof(msg)
		return w.processDeployJob(job)
	// TODO implement RESTART if still desired
	//case "RESTART":
	default:
		msg := fmt.Sprintf("unsupported action=%s", job.Action)
		log.Warnf(msg)
		return nil, errors.New(msg)
	}
}

func (w *Worker) processDeployJob(job *queue.Job) (*JobResult, error) {
	runtime := job.Request.(*queue.DeployJobRequest).Cluster.Runtime
	switch runtime {
	case "GCE":
		log.Infof("detected runtime=%s for job id=%s", runtime, job.Id)
		return w.processDeployJobGCE(job)
	case "GKE":
		log.Infof("detected runtime=%s for job id=%s", runtime, job.Id)
		return w.processDeployJobGKE(job)
	default:
		err := fmt.Errorf("unsupported runtime=%s for job id=%s", runtime, job.Id)
		return nil, err
	}
}

func (w *Worker) getConfigBall(cluster apiconfig.Cluster, version string) ([]byte, error) {
	// spin up a GCP storage client
	ctx := context.Background()
	bucketName := "arryved-app-control-config"
	pattern := fmt.Sprintf("config-app=%s,hash=.*,version=%s.tar.gz", cluster.Id.App, version)
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Errorf("Failed to create client: %v", err)
		return []byte{}, err
	}
	defer client.Close()

	// scan matching config bucket objects
	iter := client.Bucket(bucketName).Objects(ctx, nil)
	mostRecent := ""
	var max time.Time
	for {
		attrs, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Errorf("Failed to list objects: %v", err)
			return []byte{}, err
		}

		matched, err := regexp.MatchString(pattern, attrs.Name)
		if err != nil {
			log.Infof("Failed to match pattern: %v", err)
			return []byte{}, nil
		}

		// tag mostRecent seen matching object by Created
		if matched {
			if mostRecent == "" {
				mostRecent = attrs.Name
				max = attrs.Created
			} else {
				if max.Before(attrs.Created) {
					mostRecent = attrs.Name
					max = attrs.Created
				}
			}
		}
	}

	// get the contents of the mostRecent matching object
	reader, err := client.Bucket(bucketName).Object(mostRecent).NewReader(ctx)
	if err != nil {
		log.Errorf("could get configball object reader err=%s", err.Error())
		return []byte{}, err
	}
	defer reader.Close()
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Errorf("could get configball object contents err=%s", err.Error())
		return []byte{}, err
	}
	log.Infof("got configball object contents name=%s %d bytes", mostRecent, len(data))
	return data, nil
}

func (w *Worker) unzipGzip(gzipped []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(gzipped))
	if err != nil {
		log.Errorf("could get gunzip reader err=%s", err.Error())
		return nil, err
	}
	defer gzipReader.Close()
	unzipped, err := ioutil.ReadAll(gzipReader)
	if err != nil {
		log.Errorf("could not do gunzip of configball err=%s", err.Error())
		return nil, err
	}
	log.Infof("gunzipped configball %d bytes", len(unzipped))
	return unzipped, nil
}

func (w *Worker) expandTarball(tarStream []byte) (string, error) {
	// Create a temporary directory to store the expanded tarball
	tempDir, err := ioutil.TempDir("", "tempdir")
	if err != nil {
		log.Error("could not create temp dir")
		return "", err
	}

	// Create a new tar reader to read the tar stream
	tarReader := tar.NewReader(bytes.NewReader(tarStream))

	// Loop through each file in the tar stream
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			log.Infof("end of tar stream reached")
			break
		}
		if err != nil {
			log.Error("could not get next file from tar")
			return "", err
		}

		// Create a new file in the temporary directory for the current file in the tar stream
		filePath := tempDir + "/" + header.Name
		log.Infof("copying file path=%s", filePath)

		// Check if the file path ends with a "/" indicating a directory
		if strings.HasSuffix(filePath, "/") {
			// Create the directory and any necessary parent directories
			err = os.MkdirAll(filepath.Dir(filePath), 0755)
			if err != nil {
				log.Errorf("could not do recursive mkdir %s", filepath.Dir(filePath))
				return "", err
			}
			continue
		}

		// Create a new file for the current file in the tar stream
		file, err := os.Create(filePath)
		if err != nil {
			// Encountered an error while creating the file, return the error
			log.Errorf("could not do create file %s", filePath)
			return "", err
		}
		log.Infof("extracted filePath=%s", filePath)

		// Copy the contents of the file from the tar stream to the new file
		_, err = io.Copy(file, tarReader)
		if err != nil {
			// Encountered an error while copying the file, return the error
			log.Errorf("could not copy file %s", filePath)
			return "", err
		}

		// Close the file
		err = file.Close()
		if err != nil {
			// Encountered an error while closing the file, return the error
			log.Errorf("could not close file %s", filePath)
			return "", err
		}
	}

	// Return the path to the temporary directory and any errors encountered
	log.Infof("expanded config into dir=%s", tempDir)
	return tempDir, nil
}

func (w *Worker) expandConfigBall(configBall []byte) (string, error) {
	tarStream, err := w.unzipGzip(configBall)
	if err != nil {
		log.Errorf("could not close file err=%s", err.Error())
		return "", err
	}
	log.Debugf("config tar uncompressed bytes=%d", len(tarStream))
	tempDir, err := w.expandTarball(tarStream)
	if err != nil {
		return "", err
	}
	return tempDir, nil
}

func (w *Worker) compileConfig(arryvedDir string, request *queue.DeployJobRequest) (string, error) {
	region := request.Cluster.Id.Region
	variant := request.Cluster.Id.Variant

	defaultYamlPath := fmt.Sprintf("%s/config/defaults.yaml", arryvedDir)
	envYamlPath := fmt.Sprintf("%s/config/env/%s.yaml", arryvedDir, w.cfg.Env)
	regionYamlPath := fmt.Sprintf("%s/config/region/%s.yaml", arryvedDir, region)
	variantYamlPath := fmt.Sprintf("%s/config/variant/%s.yaml", arryvedDir, variant)

	defaultYaml := w.loadConfigYaml(defaultYamlPath)
	envYaml := w.loadConfigYaml(envYamlPath)
	regionYaml := w.loadConfigYaml(regionYamlPath)
	variantYaml := w.loadConfigYaml(variantYamlPath)

	appConfig, err := productconfig.MultiMerge(defaultYaml, envYaml, regionYaml, variantYaml)
	if err != nil {
		return "", err
	}
	appConfigBytes, err := yaml.Marshal(appConfig)
	if err != nil {
		return "", err
	}
	configPath := fmt.Sprintf("%s/config/config.yaml", arryvedDir)
	err = ioutil.WriteFile(configPath, appConfigBytes, 0644)
	if err != nil {
		return "", err
	}
	return configPath, nil
}

func (w *Worker) loadConfigYaml(path string) string {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		log.Warnf("could not read contents from yaml path=%s err=%s", path, err.Error())
		return ""
	}
	return string(contents)
}

func (w *Worker) wipeTempDir(rootPath string) error {
	err := os.RemoveAll(rootPath)
	if err != nil {
		return err
	}
	return nil
}

func (w *Worker) gkeApplyDeployment(arryvedDir, compiledConfigPath string, request *queue.DeployJobRequest) error {
	// If precompiled k8s (.gke) not present for env, generate k8s resources based on config/type/kind
	resourceDir := fmt.Sprintf("%s/.gke/%s", arryvedDir, w.cfg.Env)
	if _, err := os.Stat(resourceDir); os.IsNotExist(err) {
		log.Infof("resourceDir=%s does not exist, trying to generate", resourceDir)
		err := gke.GenerateFromTemplate(w.cfg, compiledConfigPath, arryvedDir, request)
		if err != nil {
			log.Errorf("could not generate files from template err=%s", err.Error())
			return err
		}
	} else {
		log.Infof("resourceDir=%s exists already", resourceDir)
	}

	// load yamls for deployment/statefulset resource
	k8sObjects, err := gke.LoadDeployYaml(resourceDir)
	if err != nil {
		err = fmt.Errorf("could not load k8s yaml objects for apply/redeploy err=%s", err.Error())
		log.Error(err)
		return err
	}

	count := len(k8sObjects)
	log.Debugf("%d k8s objects loaded", count)
	if count != 1 {
		err = fmt.Errorf("expected exactly 1 deployable object, got %d", count)
		log.Error(err)
		return err
	}
	deploymentYaml := k8sObjects[0]

	// convert object to k8s deployment and inject request version
	deployment, err := gke.DecodeYAMLToDeployment(deploymentYaml)
	if err != nil {
		err = fmt.Errorf("error while decoding deployment object err=%s", err.Error())
		log.Error(err)
		return err
	}
	image := deployment.Spec.Template.Spec.Containers[0].Image
	updatedImage := fmt.Sprintf("%s:%s", strings.Split(image, ":")[0], request.Version)
	deployment.Spec.Template.Spec.Containers[0].Image = updatedImage
	log.Infof("updated image in container spec image=%s", deployment.Spec.Template.Spec.Containers[0].Image)

	// apply deployable resource object
	return gke.ApplyDeployObject(w.cfg.KubeConfigPath, deployment)
}

//func (w *Worker) gkeApplySupportingResources(resourceDir string, request *queue.DeployJobRequest) error {
//    // load yamls for resources other than deployment, statefulset
//}

func (w *Worker) processDeployJobGKE(job *queue.Job) (*JobResult, error) {
	log.Infof("processing job id=%s as GKE deploy", job.Id)
	result := JobResult{
		ActionStatus:  "INCOMPLETE",
		ClusterStatus: "UNKNOWN",
		Detail:        "",
	}
	request := job.Request.(*queue.DeployJobRequest)

	// fetch matching config from bucket
	configBall, err := w.getConfigBall(request.Cluster, request.Version)
	if err != nil {
		log.Errorf("could not get config ball for job id=%s err=%s", job.Id, err.Error())
		return &result, err
	}

	// open tarball in mem or in temp dir
	tmpDir, err := w.expandConfigBall(configBall)
	if err != nil {
		log.Errorf("could not expand config ball for job id=%s err=%s", job.Id, err.Error())
		return &result, err
	}
	log.Debugf("temp dir created root=%s", tmpDir)
	if !w.cfg.KeepTempDir {
		defer w.wipeTempDir(tmpDir)
	}

	// Compile app config for this target
	arryvedDir := fmt.Sprintf("%s/.arryved", tmpDir)
	compiledConfigPath, err := w.compileConfig(arryvedDir, request)
	if err != nil {
		log.Errorf("could not compile config for job id=%s err=%s", job.Id, err.Error())
		return &result, err
	}
	log.Debugf("compiled config=%s", compiledConfigPath)

	// If precompiled k8s (.gke) not present for env, generate k8s resources based on config/type/kind
	if !w.kubeResourceDefsPresent(arryvedDir) {
		err = gke.GenerateFromTemplate(w.cfg, arryvedDir, compiledConfigPath, request)
		if err != nil {
			log.Errorf("no k8s resource defs for job id=%s err=%s", job.Id, err.Error())
			return &result, err
		}
	}

	// TODO apply the k8s non-pod resourcess (config, secrets, LB, etc) in async fashion; don't wait/block
	//err = w.gkeApplySupportingResources(k8sDir, request)
	//if err != nil {
	//    log.Infof("error encountered during apply/redeploy of supporting resources err=%s", err.Error())
	//}

	// apply the k8s deploy resources for the current env
	err = w.gkeApplyDeployment(arryvedDir, compiledConfigPath, request)
	if err != nil {
		log.Infof("error encountered during apply/redeploy err=%s", err.Error())
	}

	if err == nil {
		result.ActionStatus = "COMPLETE"
		result.ClusterStatus = "HEALTHY"
		result.Detail = ""
	} else {
		// TODO - clean up any failed deploy or pods
		result.ActionStatus = "FAILED"
		result.ClusterStatus = "UNHEALTHY"
		result.Detail = err.Error()
	}
	log.Infof("job id=%s processed with result=%v", job.Id, result)
	return &result, nil
}

func (w *Worker) processDeployJobGCE(job *queue.Job) (*JobResult, error) {
	log.Infof("processing job id=%s as GCE deploy", job.Id)
	result := JobResult{
		ActionStatus:  "INCOMPLETE",
		ClusterStatus: "UNKNOWN",
		Detail:        "",
	}
	request := job.Request.(*queue.DeployJobRequest)
	app := request.Cluster.Id.App
	region := request.Cluster.Id.Region
	variant := request.Cluster.Id.Variant
	version := request.Version

	// get all instances for cluster
	instanceMap, err := w.compute.GetInstancesForCluster(app, region, variant)
	if err != nil {
		msg := fmt.Sprintf("Unexpected error looking for target instances app=%s, region=%s variant=%s, err=%s", app, region, variant, err.Error())
		log.Error(msg)
		result.Detail = msg
		return &result, fmt.Errorf(msg)
	}

	batchCount := w.concurrencyToBatchCount(request.Concurrency, len(instanceMap))
	log.Infof("Deployment with concurrency of %d nodes requested against total of %d GCE instances", batchCount, len(instanceMap))

	// parallelize app-controld deploys only up to the requested concurrency's batchCount
	var wg sync.WaitGroup
	ch := make(chan struct{}, batchCount)
	for name, instance := range instanceMap {
		wg.Add(1)
		go func(name string, instance *compute.Instance) {
			defer wg.Done()
			// reserve a channel position
			ch <- struct{}{}
			defer func() {
				// release the channel position
				<-ch
			}()

			// set up timeout context
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(w.cfg.GCEDeployTimeoutS)*time.Second)
			defer cancel()

			// kick off the deployment
			log.Infof("starting deployment on instance %s for app=%s region=%s variant=%s version=%s", name, app, region, variant, version)
			result := w.gceDeploy(ctx, instance, request.Cluster.Id, version)
			log.Infof("finished deployment for=%s, result=%v", name, result)
		}(name, instance)
	}
	wg.Wait()
	// TODO return a job result w/ details as reported by app-controld (failed|succeeded)
	return nil, nil
}

func (w *Worker) concurrencyToBatchCount(concurrency string, total int) int {
	if strings.Contains(concurrency, "%") {
		percentage, err := strconv.Atoi(strings.TrimSuffix(concurrency, "%"))
		if err != nil {
			log.Fatal(err)
		}
		return int(math.Floor(float64(total) * float64(percentage) / 100))
	} else {
		batchCount, err := strconv.Atoi(concurrency)
		if err != nil {
			log.Fatal(err)
		}
		return batchCount
	}
}

func (w *Worker) gceDeploy(ctx context.Context, instance *compute.Instance, clusterId apiconfig.ClusterId, version string) *appcontrold.DeployResult {
	ch := make(chan appcontrold.DeployResult, 1)
	app := clusterId.App
	variant := clusterId.Variant

	go func(ctx context.Context, ch chan appcontrold.DeployResult) {
		log.Infof("processing deploy job for instance=%s", instance.Name)
		result := appcontrold.DeployResult{}
		psk := fmt.Sprintf("Bearer %s", readPSKFromPath(w.cfg.AppControlDPSKPath))
		url := fmt.Sprintf("%s://%s:%d/deploy?app=%s&variant=%s&version=%s",
			w.cfg.AppControlDScheme, instance.Name, w.cfg.AppControlDPort, app, variant, version)
		// TODO fix by including/referencing CA cert and issuing certs with the correct hostnames on all app-controld targets
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			msg := fmt.Sprintf("Failed to execute /deploy request to app-controld on instance=%s, err=%v", instance.Name, err)
			log.Warn(msg)
			result.Err = msg
			ch <- result
		}
		req.Header.Set("Authorization", psk)

		resp, err := client.Do(req)
		if err != nil {
			msg := fmt.Sprintf("Failed to execute /deploy request to app-controld on instance=%s, err=%v", instance.Name, err)
			log.Warn(msg)
			result.Err = msg
			ch <- result
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			msg := fmt.Sprintf("Failed body read on /status request to app-controld on instance=%s, err=%v", instance.Name, err)
			log.Warn(msg)
			result.Err = msg
			ch <- result
		}
		err = json.Unmarshal(body, &result)
		if err != nil {
			msg := fmt.Sprintf("Failed to unmarshal response from app-controld on instance=%s, body=%v, err=%v", instance.Name, string(body), err)
			log.Warn(msg)
			result.Err = msg
			ch <- result
		}
		log.Infof("finished deploy job for instance %v, result=%v", instance.Name, result)
		ch <- result
	}(ctx, ch)

	// wait for completion or timeout
	select {
	case <-ctx.Done():
		msg := fmt.Sprintf("Deployment for instance %s timed out\n", instance.Name)
		log.Warn(msg)
		return &appcontrold.DeployResult{Err: msg}
	case result := <-ch:
		log.Infof("Deployment for instance finished result=%v", result)
		return &result
	}
}

// check if .arryved/.gke has directories
func (w *Worker) kubeResourceDefsPresent(arryvedDir string) bool {
	root := fmt.Sprintf("%s/.gke", arryvedDir)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Debugf("error looking for kube resource defs err=%s", err.Error())
			return err
		}
		extension := strings.Split(info.Name(), ".")[len(strings.Split(info.Name(), "."))-1]
		if !info.IsDir() && extension == "yaml" {
			return nil
		}
		return nil
	})
	if err != nil {
		log.Debugf("kubeResourceDefsPresent root=%s err=%s", root, err.Error())
	}
	log.Debugf("kubeResourceDefsPresent root=%s", root)
	return false
}

//func (w *Worker) compileConfig(dir string, request *queue.DeployJobRequest) (string, error) {
//    log.Infof("compiling config; config dir=%s cluster=(%v,%v,%v,%v)",
//        dir, request.Cluster.Id.App, w.cfg.Env, request.Cluster.Id.Region, request.Cluster.Id.Variant)

//    defaultPath := fmt.Sprintf("%s/config/defaults.yaml", dir)
//    envPath := fmt.Sprintf("%s/config/env/%s.yaml", dir, w.cfg.Env)
//    regionPath := fmt.Sprintf("%s/config/region/%s.yaml", dir, request.Cluster.Id.Region)
//    variantPath := fmt.Sprintf("%s/config/variant/%s.yaml", dir, request.Cluster.Id.Variant)

//    defaultYaml := readFileAsString(defaultPath)
//    envYaml := readFileAsString(envPath)
//    regionYaml := readFileAsString(regionPath)
//    variantYaml := readFileAsString(variantPath)

//    appConfig, err := productconfig.MultiMerge(defaultYaml, envYaml, regionYaml, variantYaml)
//    outputPath := fmt.Sprintf("%s/config.yaml", dir)
//    compiledConfigYaml, err := yaml.Marshal(appConfig)
//    if err != nil {
//        return "", fmt.Errorf("Error marshaling compiled config err=%s", err.Error())
//    }
//    err = ioutil.WriteFile(outputPath, []byte(compiledConfigYaml), 0600)
//    if err != nil {
//        return "", fmt.Errorf("Error writing config file err=%s", err.Error())
//    }
//    log.Debugf("Wrote config file path=%s", outputPath)
//    return outputPath, nil
//}

//func readFileAsString(filename string) string {
//    data, err := ioutil.ReadFile(filename)
//    if err != nil {
//        log.Warnf("could not open file=%s", filename)
//        return ""
//    }
//    return string(data)
//}

func readPSKFromPath(pskPath string) string {
	pskFromFile, err := ioutil.ReadFile(pskPath)
	if err != nil {
		log.Warnf("couldn't read PSK from path=%s", pskPath)
		return ""
	}
	return strings.TrimSpace(string(pskFromFile))
}

func New(cfg *config.Config, jobQueue *queue.Queue, compute *gce.Client) *Worker {
	worker := Worker{
		cfg:     cfg,
		compute: compute,
		queue:   jobQueue,
	}
	return &worker
}
