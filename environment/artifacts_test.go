package environment_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	uuid "github.com/satori/go.uuid"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/smartcontractkit/helmenv/tools"
	"github.com/stretchr/testify/require"
)

func TestArtifacts(t *testing.T) {
	artifactDirectory := "test-artifacts"
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

	err = e.AddChart(&environment.HelmChart{
		ReleaseName: "chainlink",
		Path:        filepath.Join(tools.ChartsRoot, "chainlink"),
		Index:       4, // Deliberate unordered keys to test the OrderedKeys function in Charts
	})
	require.NoError(t, err)

	err = e.DeployAll()
	require.NoError(t, err)
	err = e.ConnectAll()
	require.NoError(t, err)

	err = e.Artifacts.DumpTestResult(artifactDirectory, "chainlink")
	require.NoError(t, err)

	// Overall dump dir exists
	_, err = os.Stat(artifactDirectory)
	require.NoError(t, err, fmt.Sprintf("Expected the directory '%s' to exist", artifactDirectory))

	err = filepath.WalkDir(artifactDirectory,
		func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				f, err := os.Open(path)
				if err != nil {
					require.NoError(t, err, "Error opening directory path")
					return err
				}
				defer require.NoError(t, f.Close(), "Error closing file")

				_, err = f.Readdirnames(1)
				if err != nil {
					require.NoError(t, err, fmt.Sprintf("Expected directory '%s' ton ot be empty", path))
					return err
				}
			}
			return err
		},
	)
	require.NoError(t, err)

	// Cleanup
	err = os.RemoveAll(artifactDirectory)
	require.NoError(t, err, fmt.Sprintf("Error trying to delete '%s'", artifactDirectory))
}
