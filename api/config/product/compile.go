package product

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io/ioutil"

	"github.com/arryved/app-ctrl/api/config"
)

// Compile config-2.0 from tarball
// Positional params:
//   - dir (string) directory containing uncompiled config/ dir
//   - env (string) app-control short env name
//   - id (config.ClusterId) the cluster for which the config should be compiled
//
// Returns:
//   - (string) path of the compiled config.yaml
//   - (error) error | nil
func Compile(dir, env string, id config.ClusterId) (string, error) {
	log.Infof("compiling config; config dir=%s cluster=(%v,%v,%v,%v)", dir, id.App, env, id.Region, id.Variant)

	defaultPath := fmt.Sprintf("%s/config/defaults.yaml", dir)
	envPath := fmt.Sprintf("%s/config/env/%s.yaml", dir, env)
	regionPath := fmt.Sprintf("%s/config/region/%s.yaml", dir, id.Region)
	variantPath := fmt.Sprintf("%s/config/variant/%s.yaml", dir, id.Variant)

	defaultYaml := readFileAsString(defaultPath)
	envYaml := readFileAsString(envPath)
	regionYaml := readFileAsString(regionPath)
	variantYaml := readFileAsString(variantPath)

	appConfig, err := MultiMerge(defaultYaml, envYaml, regionYaml, variantYaml)
	if err != nil {
		return "", fmt.Errorf("Error during compiling config err=%s", err.Error())
	}

	outputPath := fmt.Sprintf("%s/config.yaml", dir)
	compiledConfigYaml, err := yaml.Marshal(appConfig)
	if err != nil {
		return "", fmt.Errorf("Error marshaling compiled config err=%s", err.Error())
	}
	err = ioutil.WriteFile(outputPath, []byte(compiledConfigYaml), 0600)
	if err != nil {
		return "", fmt.Errorf("Error writing config file err=%s", err.Error())
	}
	log.Debugf("Wrote config file path=%s", outputPath)
	return outputPath, nil
}

func readFileAsString(filename string) string {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Warnf("could not open file=%s", filename)
		return ""
	}
	return string(data)
}
