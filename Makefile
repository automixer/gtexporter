APP_NAME			:= gtexporter
SRC_PATH			:= cmd/gtexporter/
BUILD_DIR			:= build
YGOT_GEN_VER		:= 'v0.29.18'
BUILD_DATE			:= $(shell date -u +%FT%TZ)
GOOS				:= $(shell go env GOOS)
GOARCH				:= $(shell go env GOARCH)
BIN_NAME 			:= $(BUILD_DIR)/$(APP_NAME)
COMMIT_ID			:= $(shell git rev-parse --short HEAD)
LDFLAGS				:= '-X main.appName=$(APP_NAME) -X main.appVersion=dev-$(COMMIT_ID) -X main.buildDate=$(BUILD_DATE)'
.DEFAULT_GOAL		:= build

install_ygot_gen:
	go install github.com/openconfig/ygot/generator@$(YGOT_GEN_VER)
.PHONY: install_ygot_gen

gen_ysocif:
	(cd pkg/datamodels/ysocif && go generate && goimports -w ./*)
.PHONY: gen_ysocif

gen_ysoclldp:
	(cd pkg/datamodels/ysoclldp && go generate && goimports -w ./*)
.PHONY: gen_ysoclldp

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
	go build -ldflags $(LDFLAGS) -o $(BIN_NAME) $(SRC_PATH)*.go
.PHONY: build

release: prepare vet
	$(eval LDFLAGS := '-X main.appName=$(APP_NAME) \
	-X main.appVersion=$(shell git describe --abbrev --tags HEAD)-$(COMMIT_ID) \
	-X main.buildDate=$(BUILD_DATE)')
	go build -ldflags $(LDFLAGS) -o $(BIN_NAME)-$(GOOS)-$(GOARCH) $(SRC_PATH)*.go
.PHONY: release

docker: prepare vet
ifeq ($(Mode),Release)
	$(eval LDFLAGS := '-X main.appName=$(APP_NAME) \
	-X main.appVersion=$(shell git describe --abbrev --tags HEAD)-$(COMMIT_ID) \
	-X main.buildDate=$(BUILD_DATE)')
endif
	go build -ldflags $(LDFLAGS) -o $(BIN_NAME) $(SRC_PATH)*.go
.PHONY: docker
