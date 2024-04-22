package gke

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
)

func LoadDeployYaml(resourceDir string) ([][]byte, error) {
	// list of yaml files in specified path
	files, err := ioutil.ReadDir(resourceDir)
	if err != nil {
		return nil, err
	}
	log.Debugf("listed %d yaml files", len(files))

	var yamls [][]byte
	for _, file := range files {
		extension := filepath.Ext(file.Name())
		if extension == ".yaml" || extension == "yml" {
			// load yaml data from file
			data, err := ioutil.ReadFile(filepath.Join(resourceDir, file.Name()))
			if err != nil {
				err = fmt.Errorf("error reading yaml file=%s err=%s", file.Name(), err.Error())
				log.Error(err)
				return nil, err
			}
			// unmarshal yaml into k8s object and add to collection
			decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
			scheme.AddToScheme(scheme.Scheme)
			obj := &unstructured.Unstructured{}
			_, _, err = decoder.Decode(data, nil, obj)
			if err != nil {
				err = fmt.Errorf("error decoding yaml file=%s err=%s", file.Name(), err.Error())
				log.Error(err)
				return nil, err
			}
			objKind := obj.GetObjectKind().GroupVersionKind().Kind
			if objKind == "Deployment" || objKind == "StatefulSet" {
				yamls = append(yamls, data)
			}
		}
	}
	log.Debugf("loaded %d kubernetes yamls", len(yamls))
	return yamls, nil
}

func ApplyDeployObject(kubeconfigPath string, deployment *v1.Deployment) error {
	log.Infof("apply/restart deploy object kubeconfig=%s", kubeconfigPath)

	// build a k8s clientset
	clientset, err := createK8sClient(kubeconfigPath)
	if err != nil {
		err = fmt.Errorf("could not create k8s client err=%s", err.Error())
		log.Error(err)
		return err
	}
	log.Debugf("k8s clientset=%v", clientset)

	// build a deploy client
	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)
	log.Debugf("k8s deploymentsClient=%v", deploymentsClient)

	// check status. if doesn't exist Create else Update
	deploymentsGet, err := deploymentsClient.Get(context.TODO(), deployment.Name, metav1.GetOptions{})
	log.Debugf("k8s result for deployments get=%v", deploymentsGet.ObjectMeta)
	if err != nil {
		log.Infof("k8s deployments get returned an error, checking if it's bad; err=%s", err.Error())
		if strings.Contains(err.Error(), "not found") {
			// Create
			log.Infof("deployment doesn't exist yet; creating deployment name=%s", deployment.Name)
			result, err := deploymentsClient.Create(context.TODO(), deployment, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("could not create deployment name=%s err=%s", deployment.Name, err.Error())
			}
			log.Infof("created deployment %q", result.GetObjectMeta().GetName())
		} else {
			err = fmt.Errorf("unhandled error getting deployment err=%s", err.Error())
			log.Error(err)
			return err
		}
	} else {
		// Update
		log.Infof("deployment already exists; procceding with update and rolling restart name=%s", deployment.Name)
		_, err = deploymentsClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
		if err != nil {
			err = fmt.Errorf("could not update deployment name=%s err=%s", deployment.Name, err.Error())
			log.Error(err)
			return err
		}
		log.Infof("deployment update succeeded name=%s", deployment.Name)
		// Rolling Restart via patch, in case it didn't happen as a consequence of the update; deploy implies at least one restart
		timestamp := time.Now().Format(time.RFC3339)
		patch := []byte(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"` + timestamp + `"}}}}}`)
		_, err = deploymentsClient.Patch(context.TODO(), deployment.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			err = fmt.Errorf("could not patch deployment for rolling restart name=%s err=%s", deployment.Name, err.Error())
			log.Error(err)
			return err
		}
		log.Infof("deployment patch for rolling update succeeded name=%s", deployment.Name)
	}

	// add hysteresis to avoid false positive for brief initial Running states
	time.Sleep(3 * time.Second)

	// wait for deploy status to settle i.e. pods are in some perm/semi-perm state
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			err = fmt.Errorf("timeout expired waiting for cluster status")
			log.Error(err)
			return err
		case <-ticker.C:
			clusterStatus, err := getClusterStatus(clientset, deployment.Name)
			if err != nil {
				err = fmt.Errorf("error getting cluster status err=%s", err.Error())
				log.Error(err)
				return err
			}
			if clusterStatus {
				log.Info("cluster settled on good status; finished")
				return nil
			}
			log.Infof("still waiting on good cluster status...")
		}
	}
}

func getClusterStatus(k8sClient *kubernetes.Clientset, deploymentName string) (bool, error) {
	podsClient := k8sClient.CoreV1().Pods("")
	pods, err := podsClient.List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploymentName),
	})
	if err != nil {
		return false, err
	}
	log.Debugf("%d pods found", len(pods.Items))

	state := true
	for i := 0; i < len(pods.Items); i++ {
		ready := pods.Items[i].Status.ContainerStatuses[0].Ready
		phase := pods.Items[i].Status.Phase
		if !ready || (phase != "Succeeded" && phase != "Running") {
			state = false
		}
	}
	return state, nil
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

func DecodeYAMLToDeployment(yamlData []byte) (*v1.Deployment, error) {
	// add the deployment type to the scheme, use a decoder that understands the type
	_ = v1.AddToScheme(scheme.Scheme)
	dec := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer()

	// decode yaml to runtime.Object
	obj, _, err := dec.Decode(yamlData, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error decoding YAML: %v", err)
	}

	// try to assert directly to *v1.Deployment, if not, try unstructured
	if deployment, ok := obj.(*v1.Deployment); ok {
		return deployment, nil
	}
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("decoded object is not a *v1.Deployment or *unstructured.Unstructured")
	}

	var deployment v1.Deployment
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), &deployment)
	if err != nil {
		return nil, fmt.Errorf("error converting to Deployment: %v", err)
	}
	return &deployment, nil
}
