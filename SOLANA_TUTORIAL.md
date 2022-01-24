#### Instantiating Solana-Chainlink cluster for tests
Install the cluster
```shell
envcli new -p examples/presets/chainlink-relay-sol.yaml
```
With created env file, connect to the environment
```shell
envcli connect -e ${CREATED_ENV_FILE}
```