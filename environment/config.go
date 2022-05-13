package environment

import (
	"errors"
	"fmt"
	"io/ioutil"
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

// MarshalSafeDuration enables proper json marshalling and unmarshaling for config values
// See: https://stackoverflow.com/questions/48050945/how-to-unmarshal-json-into-durations
type MarshalSafeDuration time.Duration

// AsTimeDuration returns as a standard time.Duration
func (d MarshalSafeDuration) AsTimeDuration() time.Duration {
	return time.Duration(d)
}

// MarshalJSON marshals durations into proper json
func (d MarshalSafeDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON unmarshals durations into proper json
func (d *MarshalSafeDuration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = MarshalSafeDuration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = MarshalSafeDuration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

func unmarshalYAML(path string, to interface{}) error {
	ap, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	log.Info().Str("Path", ap).Msg("Decoding config")
	f, err := ioutil.ReadFile(ap)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(f, to)
}

// Config represents the full configuration of an environment, it can either be defined
// programmatically at runtime, or defined in files to be used in a CLI or any other application
type Config struct {
	Path               string                           `yaml:"-" json:"-" envconfig:"config_path"`
	QPS                float32                          `yaml:"qps" json:"qps" envconfig:"qps" default:"50"`
	Burst              int                              `yaml:"burst" json:"burst" envconfig:"burst" default:"50"`
	MarshalSafeTimeout MarshalSafeDuration              `yaml:"timeout" json:"timeout" ignored:"true" default:"3m"`
	Timeout            time.Duration                    `yaml:"-" json:"-" envconfig:"timeout" default:"3m"`
	Persistent         bool                             `yaml:"persistent" json:"persistent" envconfig:"persistent"`
	NamespacePrefix    string                           `yaml:"namespace_prefix,omitempty" json:"namespace_prefix,omitempty" envconfig:"namespace_prefix"`
	Namespace          string                           `yaml:"namespace,omitempty" json:"namespace,omitempty" envconfig:"namespace"`
	Charts             Charts                           `yaml:"charts,omitempty" json:"charts,omitempty" envconfig:"charts"`
	Experiments        map[string]*chaos.ExperimentInfo `yaml:"experiments,omitempty" json:"experiments,omitempty" envconfig:"experiments"`
}

func (m *Config) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

func (m *Config) Decode(path string) error {
	// Marshal YAML first, then "envconfig" tags of that struct got marshalled
	if err := unmarshalYAML(path, &m); err != nil {
		return err
	}
	return envconfig.Process("", m)
}

// Charts represents a map of charts with some helper methods
type Charts map[string]*HelmChart

// Get returns a single Helm Chart
func (c Charts) Get(chartName string) (*HelmChart, error) {
	var chart *HelmChart
	for _, co := range c {
		if co.ReleaseName == chartName {
			chart = co
			break
		}
	}
	if chart == nil {
		return nil, fmt.Errorf("chart %s doesn't exist", chartName)
	}
	return chart, nil
}

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

// Decode is used by envconfig to initialize the custom Charts type with populated values
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

// DumpConfig dumps config to a yaml file
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
	log.Info().Str("Path", path).Str("Format", "yaml").Msg("Config file written")
	return nil
}

// DumpConfigJson dumps config to a json file
func DumpConfigJson(cfg *Config, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	d, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if _, err := f.Write(d); err != nil {
		return err
	}
	log.Info().Str("Path", path).Str("Format", "json").Msg("Config file written")
	return nil
}

// DeployOrLoadEnvironment returns a deployed environment from a given preset that can be ones pre-defined within
// the library, or passed in as part of lib usage
func DeployOrLoadEnvironment(config *Config) (*Environment, error) {
	//// Brute force way of allowing the overriding the use of an environment file without a separate function call
	envFile := os.Getenv("ENVIRONMENT_CONFIG_FILE")
	if len(envFile) > 0 {
		return DeployOrLoadEnvironmentFromConfigFile(envFile)
	}
	return deployOrLoadEnvironment(config)
}

// DeployOrLoadEnvironmentFromConfigFile returns an environment based on a preset file, mostly for use as a presets CLI
func DeployOrLoadEnvironmentFromConfigFile(configFilePath string) (*Environment, error) {
	contents, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	configFileExt := filepath.Ext(configFilePath)
	log.Info().Str("Config File", configFilePath).Str("Extension", configFileExt).Msg("Reading from config file")
	switch configFileExt {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(contents, &config); err != nil {
			return nil, err
		}
	case ".json":
		if err := json.Unmarshal(contents, &config); err != nil {
			return nil, err
		}
	default:
		err = fmt.Errorf("Invalid file extension '%s' for config file, must be yaml or json", configFileExt)
		log.Error().Str("Config File Path", configFilePath).
			Str("Extension", configFileExt).
			Err(err).
			Msg("Error reading Config File")
		return nil, err
	}

	config.Path = configFilePath
	config.Timeout = config.MarshalSafeTimeout.AsTimeDuration()
	// Always set to true when loading from file as the environment state would be lost on deployment since if false
	// config isn't written to disk
	config.Persistent = true
	return deployOrLoadEnvironment(config)
}

func deployOrLoadEnvironment(config *Config) (*Environment, error) {
	if err := envconfig.Process("", config); err != nil {
		return nil, err
	}
	if len(config.Namespace) > 0 {
		return LoadEnvironment(config)
	}
	return DeployEnvironment(config)
}
