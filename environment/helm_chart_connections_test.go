package environment_test

import (
	"fmt"
	"path/filepath"
	"testing"

	uuid "github.com/satori/go.uuid"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/smartcontractkit/helmenv/tools"
	"github.com/stretchr/testify/require"
)

// Ensures that connections are collected in consistent order, ensuring that RemoteURLs and LocalURLs match each other
func TestURLsByPort(t *testing.T) {
	envName := fmt.Sprintf("test-env-%s", uuid.NewV4().String())
	e, err := environment.NewEnvironment(&environment.Config{})
	defer teardown(t, e)
	require.NoError(t, err)
	err = e.Init(envName)
	require.NoError(t, err)

	err = e.AddChart(&environment.HelmChart{
		ReleaseName: "geth",
		Path:        filepath.Join(tools.ChartsRoot, "geth"),
		Index:       2, // Deliberate unordered keys to test the OrderedKeys function in Charts
	})
	require.NoError(t, err)

	// Launch 3 chainlink nodes
	err = e.AddChart(&environment.HelmChart{
		ReleaseName: "chainlink",
		Path:        filepath.Join(tools.ChartsRoot, "chainlink"),
		Index:       4, // Deliberate unordered keys to test the OrderedKeys function in Charts
		Values:      map[string]interface{}{"replicas": 3},
	})
	require.NoError(t, err)

	err = e.DeployAll()
	require.NoError(t, err)
	err = e.ConnectAll()
	require.NoError(t, err)

	// All other collections should match this order
	remoteURLs, err := e.Charts.Connections("chainlink").RemoteURLsByPort("access", environment.HTTP)
	require.NoError(t, err)
	localURLs, err := e.Charts.Connections("chainlink").LocalURLsByPort("access", environment.HTTP)
	require.NoError(t, err)
	// Try collecting URLs 5 times and make sure they're consistently ordered
	for i := 0; i < 5; i++ {
		newRemotes, err := e.Charts.Connections("chainlink").RemoteURLsByPort("access", environment.HTTP)
		require.NoError(t, err)
		newLocals, err := e.Charts.Connections("chainlink").LocalURLsByPort("access", environment.HTTP)
		require.NoError(t, err)
		for i := 0; i < len(remoteURLs); i++ {
			require.Equal(t, remoteURLs[i].Host, newRemotes[i].Host)
			require.Equal(t, localURLs[i].Host, newLocals[i].Host)
		}
	}

}
