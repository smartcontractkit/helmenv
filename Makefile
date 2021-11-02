BIN_DIR = bin
export GOPATH ?= $(shell go env GOPATH)
export GO111MODULE ?= on

.PHONY: lint
lint: ## run linter
	${BIN_DIR}/golangci-lint --color=always run ./... -v

.PHONY: golangci
golangci: ## install golangci-linter
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${BIN_DIR} v1.42.0

.PHONY: test
test: # run all programmatic interaction tests
	go test -v -count 1 ./...

.PHONY: install_cli
install_cli: # installs CLI
	go install cmd/cli/envcli.go