#### Helm environment
Thin wrapper for Helm to help expose different environments for local usage to help you run tests locally against multiple envs and programmatically in ephemeral envs at the same time with a nice API

#### CLI usage
Install
```
make install_cli
```
Usage docs
```
envcli -h
```

Create new environment with a preset
```
envcli -n env-1 n -p chainlink
```
You'll see config create `env-1.json`

Now you can connect
```
envcli -n env-1 c
```
You can see all forwarded ports and get it by name from config now

You can connect to all envs for which you have config

To disconnect use
```
envcli -n env-1 dc
```
To remove env use
```
envcli -n env-1 rm
```

#### Usage as a library
Have a look at tests in `environment/environment_test.go`

#### Spinning up your custom preset
If you want a custom preset that you can use only in your repo have a look at `examples`

#### Charts requirements
Your applications must have `app: *any_app_name*` label, see examples in `charts`

TODO:
- [x] Deploy a chart
- [x] Expose required port by names for every chart
- [x] Have persistent connection config for all charts
- [x] Can connect/disconnect with particular chart and all of them at once
- [x] Test cli interactions: deploy/connect/disconnect/shutdown
- [x] Minimal programmatic e2e test for deployments
- [x] Test port forwarder forking on OS X
- [ ] Test port forwarder forking on Linux
- [ ] More tests with a different charts (services/dns) to check port forwarding
- [ ] Find a way to convert k8s related config to something that devs can use with local deployment

Presets:
- [x] Chainlink <-> ETH preset
- [ ] Chainlink <-> EI <-> EA preset
- [ ] Chainlink <-> Multinode network x2 preset (reorg testing)