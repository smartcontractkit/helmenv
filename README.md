# Helm environment

Thin wrapper for Helm to help you interact with `k8s` environments

## Goals

- Create a thin wrapper around deployments that works almost like `Helm subcharts` to compose test environments, with only small code part that helps you to configure your env more easily than writing `hooks`
- Ability to use in both ephemeral deployments for CI as a lib and when creating standalone environment as a CLI

## CLI usage

Install

```sh
make install_cli
```

Usage docs

```sh
envcli -h
```

Create new environment with a preset

```sh
envcli new -p examples/presets/chainlink.yaml -o my_env.yaml
```

You'll see all deployed charts info are now added to a preset `yaml` file

Now you can connect

```sh
envcli connect -e my_env.yaml
```

You can see all forwarded ports and get it by name from config now

Dump all the logs and postgres sqls

```sh
envcli dump -e my_env.yaml -a test_logs -db chainlink
```

Apply some chaos from template

```sh
envcli chaos apply -e my_env.yaml -t examples/standalone/pod-failure-tmpl.yml
```

Now you can find running experiment ID in `examples/standalone/chainlink-example-preset`

Remove chaos by id

```sh
envcli chaos stop -p examples/standalone/chainlink-example-preset -c ${chaosID}
```

Clear all chaos if you have multiple experiments running

```sh
envcli chaos clear -e examples/standalone/chainlink-example-preset
```

To remove env use

```sh
envcli remove -e -e my_env.yaml
```

## Usage as a library

Have a look at tests in [environment/environment_test.go](environment/environment_test.go)

## Spinning up your custom preset

If you want a custom preset that you can use only in your repo have a look at [examples/programmatic](examples/programmatic)

## Charts requirements

Your applications must have `app: *any_app_name*` label, see examples in `charts`

All ports must have names, example:

```yaml
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
- [x] Chainlink <-> Relay preset
- [ ] Chainlink <-> Multinode network x2 preset (reorg testing)
