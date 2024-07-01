package worker

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	yaml "gopkg.in/yaml.v3"

	apiconfig "github.com/arryved/app-ctrl/api/config"
	productconfig "github.com/arryved/app-ctrl/api/config/product"
	"github.com/arryved/app-ctrl/api/queue"
	"github.com/arryved/app-ctrl/worker/config"
	"github.com/arryved/app-ctrl/worker/gce"
	"github.com/arryved/app-ctrl/worker/gke"
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
	log.Infof("got configball object for version=%s", mostRecent)
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
	return unzipped, nil
}

func (w *Worker) expandTarball(tarStream []byte) (string, error) {
	tempDir, err := ioutil.TempDir("", "tempdir")
	if err != nil {
		log.Error("could not create temp dir")
		return "", err
	}
	tarReader := tar.NewReader(bytes.NewReader(tarStream))
	for {
		// read next file in tar stream
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error("could not get next file from tar")
			return "", err
		}

		// create new file in temporary directory
		filePath := tempDir + "/" + header.Name
		if strings.HasSuffix(filePath, "/") {
			err = os.MkdirAll(filepath.Dir(filePath), 0755)
			if err != nil {
				log.Errorf("could not do recursive mkdir %s", filepath.Dir(filePath))
				return "", err
			}
			continue
		}
		file, err := os.Create(filePath)
		if err != nil {
			log.Errorf("could not do create file %s", filePath)
			return "", err
		}
		log.Infof("extracted filePath=%s", filePath)

		// copy contents of file from tar stream to new file
		_, err = io.Copy(file, tarReader)
		if err != nil {
			log.Errorf("could not copy file %s", filePath)
			return "", err
		}

		// close file
		err = file.Close()
		if err != nil {
			log.Errorf("could not close file %s", filePath)
			return "", err
		}
	}
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

func (w *Worker) compileConfig(tmpDir string, request *queue.DeployJobRequest) (string, error) {
	region := request.Cluster.Id.Region
	variant := request.Cluster.Id.Variant

	defaultYamlPath := fmt.Sprintf("%s/.arryved/config/defaults.yaml", tmpDir)
	envYamlPath := fmt.Sprintf("%s/.arryved/config/env/%s.yaml", tmpDir, w.cfg.Env)
	regionYamlPath := fmt.Sprintf("%s/.arryved/config/region/%s.yaml", tmpDir, region)
	variantYamlPath := fmt.Sprintf("%s/.arryved/config/variant/%s.yaml", tmpDir, variant)

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
	configPath := fmt.Sprintf("%s/.arryved/config/config.yaml", tmpDir)
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

func (w *Worker) gkeApplyDeployment(resourceDir, compiledConfigPath string, request *queue.DeployJobRequest) error {
	// If precompiled k8s (.gke) not present for env, generate k8s resources based on config/type/kind
	if _, err := os.Stat(resourceDir); os.IsNotExist(err) {
		log.Infof("resourceDir=%s does not exist, trying to generate", resourceDir)
		err := gke.GenerateFromTemplate(w.cfg, compiledConfigPath, request)
		if err != nil {
			log.Errorf("could not generate files from template err=%s", err.Error())
			return err
		}
	} else {
		log.Infof("resourceDir=%s exists already")
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
	defer w.wipeTempDir(tmpDir)

	// compile config
	compiledConfig, err := w.compileConfig(tmpDir, request)
	if err != nil {
		log.Errorf("could not compile config for job id=%s err=%s", job.Id, err.Error())
		return &result, err
	}
	log.Infof("config compiled to %s", compiledConfig)

	// apply the k8s resources for the current env
	env := w.cfg.Env
	k8sDir := fmt.Sprintf("%s/.arryved/.gke/%s", tmpDir, env)
	compiledConfig = fmt.Sprintf("%s/.arryved/config/config.yaml", tmpDir)
	log.Infof("job id=%s k8sDir=%s", job.Id, k8sDir)
	err = w.gkeApplyDeployment(k8sDir, compiledConfig, request)
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
	variant := request.Cluster.Id.Variant

	// get all instances for cluster
	instanceMap, err := w.compute.GetInstancesForCluster(app, variant)
	if err != nil {
		msg := fmt.Sprintf("Unexpected error when looking for target instances app=%s, variant=%s", app, variant)
		log.Error(msg)
		result.Detail = msg
		return &result, fmt.Errorf(msg)
	}
	log.Infof("Deployment with concurrency of %v requested against GCE instances=%v", request.Concurrency, instanceMap)

	// request app-controld deploy

	// TODO parallelize only up to the requested concurrency
	// TODO return a job result w/ details as reported by app-controld (failed|succeeded)
	log.Errorf("processDeployJobGCE not yet implemented job id=%s result=%v, request=%v", job.Id, result, request)
	return nil, nil
}

func New(cfg *config.Config, jobQueue *queue.Queue, compute *gce.Client) *Worker {
	worker := Worker{
		cfg:     cfg,
		compute: compute,
		queue:   jobQueue,
	}
	return &worker
}
