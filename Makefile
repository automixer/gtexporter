APP_NAME			:= gtexporter
APP_PATH			:= cmd/gtexporter/*.go
YGOT_GEN_VER		:= 'v0.29.18'
BUILD_DIR			:= build

BUILD_DATE			:= $(shell date -u +%FT%TZ)
GOOS				:= $(shell go env GOOS)
GOARCH				:= $(shell go env GOARCH)
APP_VERSION			:= $(shell git describe --abbrev --long --tags HEAD)
COMMIT_ID			:= $(shell git rev-parse --short HEAD)
LDFLAGS_REL			:= '-X main.appName=$(APP_NAME) -X main.appVersion=$(APP_VERSION) -X main.buildDate=$(BUILD_DATE)'
LDFLAGS_BUILD		:= '-X main.appName=$(APP_NAME) -X main.appVersion=dev-$(COMMIT_ID) -X main.buildDate=$(BUILD_DATE)'
BIN_NAME			:= $(BUILD_DIR)/$(APP_NAME)
.DEFAULT_GOAL		:= build

install_ygot_gen:
	go install github.com/openconfig/ygot/generator@$(YGOT_GEN_VER)
.PHONY: install_ygot_gen

gen_ocif:
	(cd pkg/datamodels/ocif && go generate && goimports -w ./*)
.PHONY: gen_ocif

gen_dmoclldp:
	(cd pkg/datamodels/dmoclldp && go generate && goimports -w ./*)
.PHONY: gen_dmoclldp

fmt:
	go fmt ./...
.PHONY: fmt

vet: fmt
	go vet ./...
.PHONY: vet

prepare:
	mkdir -p $(BUILD_DIR)
.PHONY: prepare

clean:
	rm -rf $(BUILD_DIR)
.PHONY: clean

build: prepare vet
	go build -ldflags $(LDFLAGS_BUILD) -o $(BIN_NAME) $(APP_PATH)
.PHONY: build

release: prepare vet
	go build -ldflags $(LDFLAGS_REL) -o $(BIN_NAME) $(APP_PATH)
.PHONY: release

docker:
ifeq ($(Mode),Release)
	make release
else
	make build
endif
.PHONY: docker
