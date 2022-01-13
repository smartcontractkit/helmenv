package environment

import "github.com/smartcontractkit/helmenv/chaos"

// ClearAllChaosStandaloneExperiments remove all chaos experiments from a standalone env
func (k *Environment) ClearAllChaosStandaloneExperiments(expInfos map[string]*chaos.ExperimentInfo) error {
	if err := k.Chaos.StopAllStandalone(expInfos); err != nil {
		return err
	}
	k.Config.Experiments = nil
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

// StopChaosStandaloneExperiment stops experiment in a standalone env
func (k *Environment) StopChaosStandaloneExperiment(expInfo *chaos.ExperimentInfo) error {
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

// ApplyChaosExperimentFromTemplate applies experiment to a standalone env
func (k *Environment) ApplyChaosExperimentFromTemplate(tmplPath string) error {
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

// ApplyChaosExperiment applies experiment to an ephemeral env
func (k *Environment) ApplyChaosExperiment(exp chaos.Experimentable) (string, error) {
	chaosName, err := k.Chaos.Run(exp)
	if err != nil {
		return chaosName, err
	}
	return chaosName, nil
}

// StopChaosExperiment stops experiment in a ephemeral env
func (k *Environment) StopChaosExperiment(id string) error {
	if err := k.Chaos.Stop(id); err != nil {
		return err
	}
	return nil
}

// ClearAllChaosExperiments clears all chaos experiments
func (k *Environment) ClearAllChaosExperiments() error {
	if err := k.Chaos.StopAll(); err != nil {
		return err
	}
	return nil
}
