package gke

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	log "github.com/sirupsen/logrus"
	productconfig "github.com/arryved/app-ctrl/api/config/product"
	"github.com/arryved/app-ctrl/api/queue"
	"github.com/arryved/app-ctrl/worker/config"
	"google.golang.org/api/iterator"
	computepb "google.golang.org/genproto/googleapis/cloud/compute/v1"
	yaml "gopkg.in/yaml.v3"
)

type AppTemplateParams struct {
	AppConfig         productconfig.AppConfig
	ControlScript     string
	CompiledConfig    string
	GCPProjectId      string
	GKEServiceAccount string
	PreSharedCert     string
	Secrets           []string
	Version           string
}

var projectIdByEnv = map[string]string{
	"cde":       "676200789565",
	"dev":       "676571955389",
	"dev-int":   "345097761962",
	"tools":     "696394560295",
	"prod":      "95123332254",
	"simc-prod": "245545856248",
	"stg":       "220668173143",
}

func GenerateFromTemplate(cfg *config.Config, compiledConfigPath string, request *queue.DeployJobRequest) error {
	var err error

	templateMap := cfg.AppTemplates
	env := cfg.Env
	kubeConfigPath := cfg.KubeConfigPath

	appConfig := productconfig.AppConfig{}
	appName := request.Cluster.Id.App
	configDir := filepath.Dir(compiledConfigPath)

	yamlFile, err := ioutil.ReadFile(compiledConfigPath)
	if err != nil {
		return nil
	}
	err = yaml.Unmarshal(yamlFile, &appConfig)
	if err != nil {
		return nil
	}
	kind := appConfig.Kind
	gkeServiceAccount, err := deriveServiceAccount(kubeConfigPath)
	if err != nil {
		err = fmt.Errorf("could not derive service account name for appName=%s config dir=%s, kind=%s", appName, configDir, kind)
		log.Error(err)
		return err
	}

	switch kind {
	case productconfig.KindOnline:
		if templatePath, ok := templateMap[string(kind)]; ok {
			err = generateOnlineResources(env, templatePath, compiledConfigPath, request.Version, gkeServiceAccount, appConfig)
		} else {
			err = fmt.Errorf("could not find template for appName=%s config dir=%s, kind=%s", appName, configDir, kind)
		}
	default:
		err = fmt.Errorf("unsupported kind for appName=%s config dir=%s, kind=%s", appName, configDir, kind)
		log.Error(err)
	}

	if err != nil {
		log.Errorf("Failed to generate k8s resource defs from template %s", err.Error())
	}
	log.Infof("generated k8s resource defs from templates; appName=%s, kind=%v", appName, kind)
	return err
}

func generateOnlineResources(env, templatePath, compiledConfigPath, version, gkeServiceAccount string, appConfig productconfig.AppConfig) error {
	configDir := filepath.Dir(compiledConfigPath)
	k8sDir := fmt.Sprintf("%s/.gke/%s", configDir, env)
	controlScriptPath := fmt.Sprintf("%s/control", configDir)
	log.Infof("rendered resources will be in k8sDir=%s", k8sDir)

	// create k8s target dir if not present
	err := os.MkdirAll(k8sDir, os.ModePerm)
	if err != nil {
		return err
	}

	// get list of template files
	files, err := ioutil.ReadDir(templatePath)
	if err != nil {
		return err
	}

	// read in the control script as a string
	controlScript, err := readFileAsString(controlScriptPath)
	if err != nil {
		return err
	}

	// read in the compiled config as a string
	compiledConfig, err := readFileAsString(compiledConfigPath)
	if err != nil {
		return err
	}

	// find the pre-shared classic TLS cert name
	preSharedCert, err := getPresharedCertName(env, appConfig.Name)
	if err != nil {
		return err
	}

	// set up config params
	params := AppTemplateParams{
		AppConfig:         appConfig,
		ControlScript:     escapeYamlString(controlScript),
		CompiledConfig:    escapeYamlString(compiledConfig),
		Version:           version,
		GKEServiceAccount: gkeServiceAccount,
		GCPProjectId:      getGCPProjectId(env),
		Secrets:           []string{"dummy"},
		PreSharedCert:     preSharedCert,
	}

	// set up any in-template util functions
	funcMap := template.FuncMap{
		"tolower": strings.ToLower,
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".yaml.tmpl") {
			inputPath := fmt.Sprintf("%s/%s", templatePath, file.Name())
			outputPath := fmt.Sprintf("%s/%s", k8sDir, strings.TrimSuffix(file.Name(), ".tmpl"))
			log.Infof("template found inputPath=%s outputPath=%s", inputPath, outputPath)
			// parse template file
			t, err := template.New(file.Name()).Funcs(funcMap).ParseFiles(inputPath)
			if t == nil {
				return fmt.Errorf("failed to parse template path=%s: err=%s", inputPath, err.Error())
			}
			// create output file
			outputHandle, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("failed to create output file path=%s err=%s nil", outputPath, err.Error())
			}
			defer outputHandle.Close()
			// apply the template against the appConfig
			err = t.ExecuteTemplate(outputHandle, file.Name(), params)
			if err != nil {
				return fmt.Errorf("failed to render template path=%s err=%s nil", inputPath, err.Error())
			}
		}
	}
	return nil
}

func escapeYamlString(s string) string {
	// Escape special characters for YAML
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\n", "\\n",
	)
	return fmt.Sprintf("\"%s\"", replacer.Replace(s))
}

func readFileAsString(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}

	fileSize := fileInfo.Size()
	buffer := make([]byte, fileSize)

	_, err = file.Read(buffer)
	if err != nil {
		return "", err
	}

	return string(buffer), nil
}

func deriveServiceAccount(kubeConfigPath string) (string, error) {
	yamlString, err := readFileAsString(kubeConfigPath)
	if err != nil {
		return "", err
	}
	var data map[string]interface{}
	err = yaml.Unmarshal([]byte(yamlString), &data)
	if err != nil {
		return "", fmt.Errorf("error parsing yaml: %v", err)
	}

	currentContext := data["current-context"].(string)
	suffix := ""
	split := strings.Split(currentContext, "_")
	if len(split) > 3 {
		suffix = split[3]
	}
	return strings.Replace(suffix, "-", "-workload-", 1), nil
}

func getGCPProjectId(env string) string {
	client := &http.Client{Timeout: 1500 * time.Millisecond} // set timeout to 1500ms
	result := projectIdByEnv[env]                            // fallback if no metadata server
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/project/numeric-project-id", nil)
	if err != nil {
		return result
	}
	req.Header.Add("Metadata-Flavor", "Google")

	resp, err := client.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return result
	}
	return string(body)
}

func getPresharedCertName(env, appName string) (string, error) {
	// Gets the latest TLS cert (classic/preshared) name via the GCP control plane API
	ctx := context.Background()
	c, err := compute.NewSslCertificatesRESTClient(ctx)
	if err != nil {
		return "", fmt.Errorf("Failed to fetch certificate: err=%s", err.Error())
	}
	defer c.Close()

	projectID := getGCPProjectId(env)
	namePattern := appName

	req := &computepb.ListSslCertificatesRequest{
		Project: projectID,
	}

	it := c.List(ctx, req)
	var certificates []*computepb.SslCertificate
	for {
		cert, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", fmt.Errorf("Failed to fetch certificate: err=%s", err.Error())
		}

		// Filter certificates by name pattern
		match, _ := regexp.MatchString(namePattern, *cert.Name)
		if match {
			certificates = append(certificates, cert)
		}
	}

	// Sort certificates by creation timestamp in descending order
	sort.Slice(certificates, func(i, j int) bool {
		creationTimeI, _ := time.Parse(time.RFC3339, *(certificates[i].CreationTimestamp))
		creationTimeJ, _ := time.Parse(time.RFC3339, *(certificates[j].CreationTimestamp))
		return creationTimeI.After(creationTimeJ)
	})

	if len(certificates) <= 0 {
		return "", fmt.Errorf("Failed to fetch certificate, no matches found for appName=%s", appName)
	}
	return *(certificates[0].Name), nil
}
