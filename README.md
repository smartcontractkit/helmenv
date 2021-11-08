#### Helm environment
Thin wrapper for Helm to help you interact with `k8s` environments

#### Goals
- Create a thin wrapper around deployments that works almost like `Helm subcharts` to compose test environments, with only small code part that helps you to configure your env more easily than writing `hooks`
- Ability to use in both ephemeral deployments for CI as a lib and when creating standalone environment as a CLI

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
envcli -p examples/standalone/chainlink-example-preset n
```
You'll see all deployed charts info are now added to a preset `yaml` file

Now you can connect
```
envcli -p examples/standalone/chainlink-example-preset c
```
You can see all forwarded ports and get it by name from config now

To disconnect use
```
envcli -p examples/standalone/chainlink-example-preset dc
```
To remove env use
```
envcli -p examples/standalone/chainlink-example-preset rm
```

#### Usage as a library
Have a look at tests in `environment/environment_test.go`

#### Spinning up your custom preset
If you want a custom preset that you can use only in your repo have a look at `examples/programmatic`

#### Charts requirements
Your applications must have `app: *any_app_name*` label, see examples in `charts`

All ports must have names, example:
```
ports:
    - name: http-rpc
      containerPort: 8544
```

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
- [x] Test config interactions and overrides for viper and Helm values

Presets:
- [x] Chainlink <-> ETH preset
- [ ] Chainlink <-> EI <-> EA preset
- [ ] Chainlink <-> Multinode network x2 preset (reorg testing)