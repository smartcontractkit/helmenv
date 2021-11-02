#### Helm environment
Thin wrapper for Helm to help expose different environments for local usage to help you run tests locally against multiple envs and programmatically in ephemeral envs at the same time with a nice API

TODO:
- [x] Deploy a chart
- [x] Expose required port by names for every chart
- [x] Have persistent connection config for all charts
- [x] Can connect/disconnect with particular chart and all of them at once
- [x] Test cli interactions: deploy/connect/disconnect/shutdown
- [x] Test programmatic API

#### CLI usage
Install
```
make install_cli
```

Create new environment
```
envcli -n env-1 new
```
You'll see config create `env-1.json`

Now you can connect
```
envcli -n env-1 connect
```
You can see all forwarded ports and get it by name from config now
You can connect to all envs for which you have config

To disconnect use
```
envcli -n env-1 disconnect
```
To shutdown env use
```
envcli -n env-1 shutdown
```

#### Usage as a library
Have a look at tests in `environment/environment_test.go`