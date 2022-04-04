package environment_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/smartcontractkit/helmenv/environment"
	"github.com/stretchr/testify/require"
)

func TestChartsFile(t *testing.T) {
	t.Parallel()

	chartsTestFilePath := "./charts-test-file.json"
	chainlinkImage := "test/chainlink/image"
	chainlinkVersion := "v0.6.4"
	gethImage := "test/geth/image"
	gethVersion := "v0.6.5"

	chartsTestFile, err := os.Create(chartsTestFilePath)
	require.NoError(t, err)
	defer func() { // Cleanup after test
		require.NoError(t, chartsTestFile.Close(), "Error closing test charts file")
		if _, err = os.Stat(chartsTestFilePath); err == nil {
			require.NoError(t, os.Remove(chartsTestFilePath), "Error deleting test charts file")
		}
	}()

	_, err = chartsTestFile.WriteString(fmt.Sprintf(`{
		"geth":{
			"values":{
				 "geth":{
						"image":{
							 "image":"%s",
							 "version":"%s"
						}
				 }
			}
	 },
		"chainlink":{
			 "values":{
					"chainlink":{
						 "image":{
								"image":"%s",
								"version":"%s"
						 }
					}
			 }
		}
	}`, gethImage, gethVersion, chainlinkImage, chainlinkVersion))
	require.NoError(t, err)
	err = chartsTestFile.Sync()
	require.NoError(t, err)

	chainlinkConfig := environment.NewChainlinkConfig(map[string]interface{}{}, "")
	err = chainlinkConfig.Charts.Decode(chartsTestFilePath)
	require.NoError(t, err)
}
