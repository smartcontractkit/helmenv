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
envcli new -p examples/standalone/chainlink-example-preset
```
You'll see all deployed charts info are now added to a preset `yaml` file

Now you can connect
```
envcli connect -e examples/standalone/chainlink-example-preset
```
You can see all forwarded ports and get it by name from config now

Dump all the logs and postgres sqls
```
envcli dump -e chainlink-example-preset -a test_logs -db chainlink
```
Apply some chaos from template
```
envcli chaos apply -e examples/standalone/chainlink-example-preset -t examples/standalone/pod-failure-tmpl.yml
```
Now you can find running experiment ID in `examples/standalone/chainlink-example-preset`

Remove chaos by id
```
envcli chaos stop -p examples/standalone/chainlink-example-preset -c ${chaosID}
```
Clear all chaos if you have multiple experiments running
```
envcli chaos clear -e examples/standalone/chainlink-example-preset
```

To disconnect use
```
envcli disconnect -e examples/standalone/chainlink-example-preset
```
To remove env use
```
envcli remove -e examples/standalone/chainlink-example-preset
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
- [x] Test port forwarder forking on Linux
- [ ] More tests with a different charts (services/dns) to check port forwarding
- [x] Test config interactions and overrides for viper and Helm values

Presets:
- [x] Chainlink <-> ETH preset
- [ ] Chainlink <-> EI <-> EA preset
- [ ] Chainlink <-> Multinode network x2 preset (reorg testing)