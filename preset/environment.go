package preset

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/smartcontractkit/helmenv/tools"
	"path/filepath"
)

// UsePreset spins up a preset environment
func UsePreset(name string, cfg *environment.Config) (*environment.Environment, error) {
	switch name {
	case "chainlink":
		return NewChainlinkEnv(cfg)
	case "terra-chainlink":
		return NewTerraChainlink(cfg)
	default:
		return nil, fmt.Errorf("no suitable environment preset found for %s", name)
	}
}

// NewTerraChainlink new chainlink env with LocalTerra blockchain
func NewTerraChainlink(cfg *environment.Config) (*environment.Environment, error) {
	e, err := environment.NewEnvironment(cfg)
	if err != nil {
		return nil, err
	}
	if err := e.Init(); err != nil {
		return nil, err
	}
	if err := e.AddChart(&environment.ChartSettings{
		ReleaseName: "localterra",
		Path:        filepath.Join(tools.ChartsRoot, "localterra"),
		Values:      nil,
	}); err != nil {
		return nil, err
	}
	// TODO: awaiting ability to setup IC/CI creds via API, otherwise we have a circular dependency  in deployment
	//if err := e.AddChart(&environment.ChartSettings{
	//	ReleaseName: "terra-relay",
	//	Path:        filepath.Join(tools.ChartsRoot, "terra-relay"),
	//	Values:      nil,
	//}); err != nil {
	//	return nil, err
	//}
	if err := e.AddChart(&environment.ChartSettings{
		ReleaseName: "chainlink",
		Path:        filepath.Join(tools.ChartsRoot, "chainlink"),
		Values: map[string]interface{}{
			"replicaCount": 2,
		},
	}); err != nil {
		return nil, err
	}
	if err := e.DeployAll(); err != nil {
		if err := e.Teardown(); err != nil {
			return nil, errors.Wrapf(err, "failed to shutdown namespace")
		}
		return nil, err
	}
	return e, nil
}

// NewChainlinkEnv creates new chainlink environment
func NewChainlinkEnv(cfg *environment.Config) (*environment.Environment, error) {
	e, err := environment.NewEnvironment(cfg)
	if err != nil {
		return nil, err
	}
	if err := e.Init(); err != nil {
		return nil, err
	}
	if err := e.AddChart(&environment.ChartSettings{
		ReleaseName: "geth",
		Path:        filepath.Join(tools.ChartsRoot, "geth"),
		Values:      nil,
	}); err != nil {
		return nil, err
	}
	if err := e.AddChart(&environment.ChartSettings{
		ReleaseName: "chainlink",
		Path:        filepath.Join(tools.ChartsRoot, "chainlink"),
		Values: map[string]interface{}{
			"replicaCount": 3,
		},
	}); err != nil {
		return nil, err
	}
	if err := e.DeployAll(); err != nil {
		if err := e.Teardown(); err != nil {
			return nil, errors.Wrapf(err, "failed to shutdown namespace")
		}
		return nil, err
	}
	return e, nil
}
