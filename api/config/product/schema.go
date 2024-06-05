package product

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v3"
	"reflect"
	"strings"
)

type Kind string
type Repo string
type Runtime string

const (
	KindOnline   Kind = "online"
	KindInternal Kind = "internal"
	KindBatch    Kind = "batch"
	KindCron     Kind = "cron"
	KindApp      Kind = "app"

	RepoApt       Repo = "apt"
	RepoMvn       Repo = "mvn"
	RepoPypi      Repo = "pypi"
	RepoContainer Repo = "container"
	// backwards compatibility only
	RepoGCE Repo = "gce"
	RepoGKE Repo = "gke"

	RuntimeGCE     Runtime = "GCE"
	RuntimeGKE     Runtime = "GKE"
	RuntimeGCF     Runtime = "GCF"
	RuntimeMobile  Runtime = "mobile"
	RuntimeLib     Runtime = "lib"
	RuntimeDesktop Runtime = "desktop"
)

type Schemaless struct {
	Value interface{}
}

func (s Schemaless) MarshalYAML() (interface{}, error) {
	return marshalWithoutValueKey(s.Value)
}

func marshalWithoutValueKey(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			marshaledVal, err := marshalWithoutValueKey(val)
			if err != nil {
				return nil, err
			}
			result[key] = marshaledVal
		}
		return result, nil
	case []interface{}:
		var result []interface{}
		for _, val := range v {
			marshaledVal, err := marshalWithoutValueKey(val)
			if err != nil {
				return nil, err
			}
			result = append(result, marshaledVal)
		}
		return result, nil
	default:
		return v, nil
	}
}

func (s *Schemaless) UnmarshalYAML(value *yaml.Node) error {
	// pass deletion tag to value, so it can be deleted post-merge
	if value.Tag == "!DELETE" {
		s.Value = "!DELETE"
		return nil
	}
	switch value.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		var items []Schemaless
		for _, item := range value.Content {
			var schemalessItem Schemaless
			if err := item.Decode(&schemalessItem); err != nil {
				return err
			}
			items = append(items, schemalessItem)
		}
		s.Value = items
	case yaml.MappingNode:
		m := make(map[string]Schemaless)
		for i := 0; i < len(value.Content); i += 2 {
			key := value.Content[i].Value
			var schemalessValue Schemaless
			if err := value.Content[i+1].Decode(&schemalessValue); err != nil {
				return err
			}
			m[key] = schemalessValue
		}
		s.Value = m
	default:
		var v interface{}
		if err := value.Decode(&v); err != nil {
			return err
		}
		s.Value = v
	}
	return nil
}

type AppConfig struct {
	// metadata fields that are really only sourced from defaults (overrides should be ignored)
	Name     string                 `yaml:"name"`
	Version  string                 `yaml:"version"`
	Control  string                 `yaml:"control"`
	Port     *int                   `yaml:"port,omitempty"`
	Kind     Kind                   `yaml:"kind"`
	Runtime  Runtime                `yaml:"runtime"`
	RepoType Repo                   `yaml:"repo_type,omitempty"`
	RepoName string                 `yaml:"repo_name,omitempty"`
	Mvn      map[string]interface{} `yaml:"mvn,omitempty"`

	// fields that are actually override-merged, including env and files
	Other map[string]Schemaless `yaml:",inline"`
}

func (k *Kind) UnmarshalYAML(value *yaml.Node) error {
	var kind string
	err := value.Decode(&kind)
	if err != nil {
		return err
	}
	switch kind {
	case string(KindOnline), string(KindInternal), string(KindBatch), string(KindCron), string(KindApp):
		*k = Kind(kind)
		return nil
	default:
		return fmt.Errorf("invalid kind: %s", kind)
	}
}

func (r *Runtime) UnmarshalYAML(value *yaml.Node) error {
	var runtime string
	err := value.Decode(&runtime)
	if err != nil {
		return err
	}
	switch runtime {
	case string(RuntimeGCE), string(RuntimeGKE), string(RuntimeGCF), string(RuntimeMobile), string(RuntimeLib), string(RuntimeDesktop):
		*r = Runtime(runtime)
		return nil
	default:
		return fmt.Errorf("invalid runtime: %s", runtime)
	}
}

func (r *Repo) UnmarshalYAML(value *yaml.Node) error {
	var repo string
	if err := value.Decode(&repo); err != nil {
		return err
	}
	switch repo {
	case string(RepoGCE), string(RepoApt):
		*r = RepoApt
		return nil
	case string(RepoGKE), string(RepoContainer):
		*r = RepoContainer
		return nil
	case string(RepoMvn), string(RepoPypi):
		*r = Repo(repo)
		return nil
	default:
		return fmt.Errorf("invalid repo type: %s", repo)
	}
}

func ParseYaml(yamlBytes []byte) (*AppConfig, error) {
	var appConfig AppConfig
	decoder := yaml.NewDecoder(strings.NewReader(string(yamlBytes)))
	decoder.KnownFields(true)
	err := decoder.Decode(&appConfig)
	return &appConfig, err
}

func stripDeletedKeys(data map[string]Schemaless) {
	for key, schemaless := range data {
		switch v := schemaless.Value.(type) {
		case map[string]Schemaless:
			// Recursively walk through nested maps
			stripDeletedKeys(v)
		case string:
			// If the value is "!DELETE", delete the key (expects tags to be converted to this value during unmarshal)
			if v == "!DELETE" {
				delete(data, key)
			}
		}
	}
}

type customTransformer struct{}

func (t customTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	return func(dst, src reflect.Value) error {
		if !dst.CanSet() {
			log.Infof("BW can't set")
			return nil
		}
		if dst.IsZero() {
			log.Infof("BW is zero")
			return nil
		}
		log.Infof("BW setting")
		dst.Set(src)
		return nil
	}
}

func mergeMaps(dst, src map[string]Schemaless) {
	for key, srcVal := range src {
		if dstVal, ok := dst[key]; ok {
			switch dstValTyped := dstVal.Value.(type) {
			case map[string]Schemaless:
				if srcValTyped, ok := srcVal.Value.(map[string]Schemaless); ok {
					mergeMaps(dstValTyped, srcValTyped)
					dst[key] = Schemaless{Value: dstValTyped}
				} else {
					dst[key] = srcVal
				}
			default:
				dst[key] = srcVal
			}
		} else {
			dst[key] = srcVal
		}
	}
}

func Merge(base *AppConfig, override AppConfig) error {
	mergeMaps(base.Other, override.Other)
	// Get rid of any keys with value !DELETE before merging;
	// Use a second pass b/c mergo doesn't support target/base key deletion in transforms
	stripDeletedKeys(base.Other)
	return nil
}

func MultiMerge(defaultYaml, envYaml, regionYaml, variantYaml string) (*AppConfig, error) {
	if defaultYaml == "" {
		return nil, fmt.Errorf("must provide a default yaml at minimum")
	}

	defaultLayer, err := ParseYaml([]byte(defaultYaml))
	if err != nil {
		return nil, fmt.Errorf("could not parse default layer err=%s", err.Error())
	}

	if envYaml != "" {
		envLayer, err := ParseYaml([]byte(envYaml))
		if err != nil {
			return nil, fmt.Errorf("could not parse env layer err=%s", err.Error())
		}
		err = Merge(defaultLayer, *envLayer)
		if err != nil {
			return nil, fmt.Errorf("could not merge env layer err=%s", err.Error())
		}
	}

	if regionYaml != "" {
		regionLayer, err := ParseYaml([]byte(regionYaml))
		if err != nil {
			return nil, fmt.Errorf("could not parse region layer err=%s", err.Error())
		}
		err = Merge(defaultLayer, *regionLayer)
		if err != nil {
			return nil, fmt.Errorf("could not merge region layer err=%s", err.Error())
		}
	}

	if variantYaml != "" {
		variantLayer, err := ParseYaml([]byte(variantYaml))
		if err != nil {
			return nil, fmt.Errorf("could not parse variant layer err=%s", err.Error())
		}
		err = Merge(defaultLayer, *variantLayer)
		if err != nil {
			return nil, fmt.Errorf("could not merge variant layer err=%s", err.Error())
		}
	}

	return defaultLayer, nil
}
