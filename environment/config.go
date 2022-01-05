package environment

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/imdario/mergo"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/helmenv/chaos"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/json"
)

// Config represents the full configuration of an environment, it can either be defined
// programmatically at runtime, or defined in files to be used in a CLI or any other application
type Config struct {
	Path            string                           `yaml:"-" json:"-" envconfig:"config_path"`
	QPS             float32                          `yaml:"qps" json:"qps" envconfig:"qps" default:"50"`
	Burst           int                              `yaml:"burst" json:"burst" envconfig:"burst" default:"50"`
	Timeout         time.Duration                    `yaml:"timeout" json:"timeout" envconfig:"timeout" default:"3m"`
	Persistent      bool                             `yaml:"persistent" json:"persistent" envconfig:"persistent"`
	NamespacePrefix string                           `yaml:"namespace_prefix,omitempty" json:"namespace_prefix,omitempty" envconfig:"namespace_prefix"`
	Namespace       string                           `yaml:"namespace,omitempty" json:"namespace,omitempty" envconfig:"namespace"`
	Charts          Charts                           `yaml:"charts,omitempty" json:"charts,omitempty" envconfig:"charts"`
	Experiments     map[string]*chaos.ExperimentInfo `yaml:"experiments,omitempty" json:"experiments,omitempty" envconfig:"experiments"`
}

// Charts represents a map of charts with some helper methods
type Charts map[string]*HelmChart

// Connections is a helper method for simply accessing chart connections, also safely allowing method chaining
func (c Charts) Connections(chart string) *ChartConnections {
	if chart, ok := c[chart]; !ok {
		return &ChartConnections{}
	} else {
		return &chart.ChartConnections
	}
}

// ExecuteInPod is similar to kubectl exec
func (c Charts) ExecuteInPod(chartName string, podNameSubstring string, podIndex int, containerName string, command []string) error {
	chart, ok := c[chartName]
	if !ok {
		return fmt.Errorf("no chart with name %s", chartName)
	}
	pods, err := chart.GetPodsByNameSubstring(podNameSubstring)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return fmt.Errorf("no pods with name that contain %s", podNameSubstring)
	}
	if podIndex >= len(pods) || podIndex < 0 {
		return errors.New("pod index is out bounds")
	}
	_, _, err = chart.ExecuteInPod(pods[podIndex].Name, containerName, command)
	if err != nil {
		return err
	}
	return nil
}

// Decode is used by envconfig to initialise the custom Charts type with populated values
// This function will take a JSON object representing charts, and unmarshal it into the existing object to "merge" the
// two
func (c Charts) Decode(value string) error {
	// Support the use of files for unmarshaling charts JSON
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(workingDir, value)); err == nil {
		log.Debug().Str("Filepath", filepath.Join(workingDir, value)).Msg("Reading Chart values from file")
		b, err := os.ReadFile(value)
		if err != nil {
			return err
		}
		value = string(b)
	}
	charts := Charts{}
	if err := json.Unmarshal([]byte(value), &charts); err != nil {
		return fmt.Errorf("failed to unmarshal JSON, either a file path specific doesn't exist, or the JSON is invalid: %v", err)
	}
	return mergo.Merge(&c, charts, mergo.WithOverride)
}

// OrderedKeys returns an ordered list of the map keys based on the charts Index value
func (c Charts) OrderedKeys() [][]string {
	keys := make([][]string, len(c))
	indexMap := map[int][]string{}
	for key, chart := range c {
		indexMap[chart.Index] = append(indexMap[chart.Index], key)
	}
	var indexes []int
	for index := range indexMap {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for i, chartIndex := range indexes {
		keys[i] = indexMap[chartIndex]
	}
	return keys
}

// DumpConfig dumps config to a file
func DumpConfig(cfg *Config, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	d, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if _, err := f.Write(d); err != nil {
		return err
	}
	return nil
}

// DeployOrLoadEnvironment returns a deployed environment from a given preset that can be ones pre-defined within
// the library, or passed in as part of lib usage
func DeployOrLoadEnvironment(config *Config, chartDirectory string) (*Environment, error) {
	// Brute force way of allowing the overriding the use of an environment file without a separate function call
	envFile := os.Getenv("ENVIRONMENT_FILE")
	if len(envFile) > 0 {
		return DeployOrLoadEnvironmentFromConfigFile(chartDirectory, envFile)
	}
	return deployOrLoadEnvironment(config, chartDirectory)
}

// DeployOrLoadEnvironmentFromConfigFile returns an environment based on a preset file, mostly for use as a presets CLI
func DeployOrLoadEnvironmentFromConfigFile(chartDirectory, filePath string) (*Environment, error) {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	if err := yaml.Unmarshal(contents, &config); err != nil {
		return nil, err
	}
	config.Path = filePath
	// Always set to true when loading from file as the environment state would be lost on deployment since if false
	// config isn't written to disk
	config.Persistent = true
	return deployOrLoadEnvironment(config, chartDirectory)
}

func deployOrLoadEnvironment(config *Config, chartDirectory string) (*Environment, error) {
	if err := envconfig.Process("", config); err != nil {
		return nil, err
	}
	if len(config.Namespace) > 0 {
		return LoadEnvironment(config)
	}
	return DeployEnvironment(config, chartDirectory)
}
