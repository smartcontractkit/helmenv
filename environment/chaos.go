package environment

import "github.com/smartcontractkit/helmenv/chaos"

// ClearAllStandaloneExperiments remove all chaos experiments from a presets env
func (k *Environment) ClearAllStandaloneExperiments(expInfos map[string]*chaos.ExperimentInfo) error {
	if err := k.Chaos.StopAllStandalone(expInfos); err != nil {
		return err
	}
	k.Config.Experiments = nil
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

// StopExperimentStandalone stops experiment in a presets env
func (k *Environment) StopExperimentStandalone(expInfo *chaos.ExperimentInfo) error {
	if err := k.Chaos.StopStandalone(expInfo); err != nil {
		return err
	}
	k.Config.Experiments[expInfo.Name] = nil
	if len(k.Config.Experiments) == 0 {
		k.Config.Experiments = nil
	}
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

// ApplyExperimentStandalone applies experiment to a presets env
func (k *Environment) ApplyExperimentStandalone(tmplPath string) error {
	expInfo, err := k.Chaos.RunTemplate(tmplPath)
	if err != nil {
		return err
	}
	k.Config.Experiments[expInfo.Name] = expInfo
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

// ApplyExperiment applies experiment to an ephemeral env
func (k *Environment) ApplyExperiment(exp chaos.Experimentable) (string, error) {
	chaosName, err := k.Chaos.Run(exp)
	if err != nil {
		return chaosName, err
	}
	return chaosName, nil
}

// StopExperiment stops experiment in a ephemeral env
func (k *Environment) StopExperiment(chaosName string) error {
	if err := k.Chaos.Stop(chaosName); err != nil {
		return err
	}
	return nil
}
