BIN_DIR = bin
export GOPATH ?= $(shell go env GOPATH)
export GO111MODULE ?= on

LINUX=LINUX
OSX=OSX
WINDOWS=WIN32
OSFLAG :=
ifeq ($(OS),Windows_NT)
	OSFLAG = $(WINDOWS)
else
	UNAME_S := $(shell uname -s)
	ifeq ($(UNAME_S),Linux)
		OSFLAG = $(LINUX)
	endif
	ifeq ($(UNAME_S),Darwin)
		OSFLAG = $(OSX)
	endif
endif

.PHONY: lint
lint: ## run linter
	golangci-lint --color=always run ./... -v

.PHONY: test
test: # run all programmatic interaction tests
	go test -v -p 5 ./...

.PHONY: install_cli
install_cli: # installs CLI
	go install cmd/cli/envcli.go

install_tools:
ifeq ($(OSFLAG),$(WINDOWS))
	echo "If you are running windows and know how to install what is needed, please contribute by adding it here!"
	exit 1
endif
ifeq ($(OSFLAG),$(LINUX))
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${BIN_DIR} v$(shell cat ./.tool-versions | grep golangci-lint | sed -En "s/golangci-lint.(.*)/\1/p")
	# TODO: golang, k3d, act
endif
ifeq ($(OSFLAG),$(OSX))
	brew install asdf
	asdf plugin-add golang https://github.com/kennyp/asdf-golang.git || true
	asdf plugin add k3d https://github.com/spencergilbert/asdf-k3d.git || true
	asdf plugin add act https://github.com/grimoh/asdf-act.git || true
	asdf plugin add golangci-lint https://github.com/hypnoglow/asdf-golangci-lint.git || true
	asdf install
endif
