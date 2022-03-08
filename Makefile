SHELL=/bin/bash
.SHELLFLAGS=-euo pipefail -c

# Dependency Versions
CONTROLLER_GEN_VERSION:=v0.6.2
OLM_VERSION:=v0.20.0
KIND_VERSION:=v0.11.1
YQ_VERSION:=v4@v4.12.0
GOIMPORTS_VERSION:=v0.1.5
GOLANGCI_LINT_VERSION:=v1.43.0
OPM_VERSION:=v1.18.0

# Build Flags
export CGO_ENABLED:=0
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
SHORT_SHA=$(shell git rev-parse --short HEAD)
VERSION?=${SHORT_SHA}
BUILD_DATE=$(shell date +%s)
MODULE:=github.com/openshift/addon-operator
GOFLAGS=
LD_FLAGS=-X $(MODULE)/internal/version.Version=$(VERSION) \
			-X $(MODULE)/internal/version.Branch=$(BRANCH) \
			-X $(MODULE)/internal/version.Commit=$(SHORT_SHA) \
			-X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

UNAME_OS:=$(shell uname -s)
UNAME_OS_LOWER:=$(shell uname -s | awk '{ print tolower($$0); }') # UNAME_OS but in lower case
UNAME_ARCH:=$(shell uname -m)

# PATH/Bin
PROJECT_DIR:=$(shell pwd)
DEPENDENCIES:=.deps
DEPENDENCY_BIN:=$(abspath $(DEPENDENCIES)/bin)
DEPENDENCY_VERSIONS:=$(abspath $(DEPENDENCIES)/$(UNAME_OS)/$(UNAME_ARCH)/versions)
export PATH:=$(DEPENDENCY_BIN):$(PATH)

# Config
KIND_KUBECONFIG_DIR:=.cache/integration
KIND_KUBECONFIG:=$(KIND_KUBECONFIG_DIR)/kubeconfig
export KUBECONFIG?=$(abspath $(KIND_KUBECONFIG))
export GOLANGCI_LINT_CACHE=$(abspath .cache/golangci-lint)
export SKIP_TEARDOWN?=
KIND_CLUSTER_NAME:="addon-operator" # name of the kind cluster for local development.
ENABLE_API_MOCK?="false"
ENABLE_WEBHOOK?="false"
ENABLE_MONITORING?="true"
WEBHOOK_PORT?=8080

# Container
IMAGE_ORG?=quay.io/app-sre
ADDON_OPERATOR_MANAGER_IMAGE?=$(IMAGE_ORG)/addon-operator-manager:$(VERSION)
ADDON_OPERATOR_WEBHOOK_IMAGE?=$(IMAGE_ORG)/addon-operator-webhook:$(VERSION)
API_MOCK_IMAGE?=$(IMAGE_ORG)/api-mock:$(VERSION)

# COLORS
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET  := $(shell tput -Txterm sgr0)

# ---------
##@ General
# ---------

# Default build target - must be first!
all:
	./mage build:all

## Display this help.
help:
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@awk \
	'/^[^[:space:]]+:/ { \
		helpMessage = match(lastLine, /^## (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 3, RLENGTH); \
			printf "  ${GREEN}%-30s${RESET}%s\n", helpCommand, helpMessage; \
		} \
	} \
	/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

## Prints version as used by build commands.
version:
	@echo $(VERSION)
.PHONY: version

## Cleans cached binaries, dependencies and container image tars.
clean: delete-kind-cluster
	@rm -rf bin .cache
.PHONY: clean

# ---------
##@ Compile
# ---------

# empty force target to ensure a target always executes.
FORCE:

# ----------------------------
# Dependencies (project local)
# ----------------------------

kind:
	./mage dependency:kind

yq:
	./mage dependency:yq

golangci-lint:
	./mage dependency:golangcilint

opm:
	./mage dependency:opm

helm:
	./mage dependency:helm

## Run go mod tidy in all go modules
tidy:
	@cd apis; go mod tidy
	@go mod tidy

# ------------
##@ Generators
# ------------

## Generate deepcopy code, kubernetes manifests and docs.
generate: openshift-ci-test-build
	./mage generate:all
.PHONY: generate

# Makes sandwich
# https://xkcd.com/149/
sandwich:
ifneq ($(shell id -u), 0)
	@echo "What? Make it yourself."
else
	@echo "Okay."
endif
.PHONY: sandwich

# ---------------------
##@ Testing and Linting
# ---------------------

## Runs code-generators, checks for clean directory and lints the source code.
lint:
	./mage test:lint
.PHONY: lint

## Runs code-generators and unittests.
test-unit: generate
	@echo "running unit tests..."
	./mage test:unit
.PHONY: test-unit

## Runs the Integration testsuite against the current $KUBECONFIG cluster
test-integration: export ENABLE_WEBHOOK=true
test-integration: export ENABLE_API_MOCK=true
test-integration:
	@echo "running integration tests..."
	@go test -v -count=1 -timeout=20m ./integration/...
.PHONY: test-integration

# legacy alias for CI/CD
test-e2e: | \
	config/deploy/deployment.yaml \
	config/deploy/api-mock/deployment.yaml \
	config/deploy/webhook/deployment.yaml \
	test-integration
.PHONY: test-e2e

## Runs the Integration testsuite against the current $KUBECONFIG cluster. Skips operator setup and teardown.
test-integration-short:
	@echo "running [short] integration tests..."
	@go test -v -count=1 -short ./integration/...

# make sure that we install our components into the kind cluster and disregard normal $KUBECONFIG
test-integration-local: export KUBECONFIG=$(abspath $(KIND_KUBECONFIG))
## Setup a local dev environment and execute the full integration testsuite against it.
test-integration-local: | \
	dev-setup \
	prepare-addon-operator \
	prepare-addon-operator-webhook \
	prepare-api-mock \
	test-integration
.PHONY: test-integration-local

# -------------------------
##@ Development Environment
# -------------------------

## Installs all project dependencies into $(PWD)/.deps/bin
dependencies:
	./mage dependency:all
.PHONY: dependencies

## Run cmd/addon-operator-manager against $KUBECONFIG.
run-addon-operator-manager:

## Run cmd/% against $KUBECONFIG.
run-%: generate
	go run -ldflags "-w $(LD_FLAGS)" \
		./cmd/$*/main.go \
			-pprof-addr="127.0.0.1:8065" \
			-metrics-addr="0"

# make sure that we install our components into the kind cluster and disregard normal $KUBECONFIG
dev-setup: export KUBECONFIG=$(abspath $(KIND_KUBECONFIG))
## Setup a local env for feature development. (Kind, OLM, OKD Console)
dev-setup: | \
	create-kind-cluster \
	setup-olm \
	setup-okd-console
.PHONY: dev-setup

## Setup a local env for integration test development. (Kind, OLM, OKD Console, Addon Operator). Use with test-integration-short.
test-setup: | \
	dev-setup \
	setup-addon-operator
.PHONY: test-setup

## Creates an empty kind cluster to be used for local development.
create-kind-cluster: kind
	@echo "creating kind cluster addon-operator..."
	@mkdir -p .cache/integration
	@(source hack/determine-container-runtime.sh; \
		mkdir -p $(KIND_KUBECONFIG_DIR); \
		$$KIND_COMMAND create cluster \
			--kubeconfig=$(KIND_KUBECONFIG) \
			--name=$(KIND_CLUSTER_NAME); \
		echo; \
	) 2>&1 | sed 's/^/  /'
	@if [[ ! -O "$(dir KIND_KUBECONFIG)" ]]; then \
		sudo chown -R $$USER: "$(KIND_KUBECONFIG)"; \
	fi
	@if [[ ! -O "$(KIND_KUBECONFIG)" ]]; then \
		sudo chown $$USER: "$(KIND_KUBECONFIG)"; \
	fi

	@echo "post-setup for kind-cluster..."
	@(kubectl create -f config/ocp/cluster-version-operator_01_clusterversion.crd.yaml; \
		kubectl create -f config/ocp/config-operator_01_proxy.crd.yaml; \
		kubectl create -f config/ocp/cluster-version.yaml; \
		kubectl create -f config/ocp/monitoring.coreos.com_servicemonitors.yaml; \
		echo; \
	) 2>&1 | sed 's/^/  /'
.PHONY: create-kind-cluster

## Deletes the previously created kind cluster.
delete-kind-cluster: kind
	@echo "deleting kind cluster addon-operator..."
	@(source hack/determine-container-runtime.sh; \
		$$KIND_COMMAND delete cluster \
			--kubeconfig="$(KIND_KUBECONFIG)" \
			--name=$(KIND_CLUSTER_NAME); \
		rm -rf "$(KIND_KUBECONFIG)"; \
		echo; \
	) 2>&1 | sed 's/^/  /'
.PHONY: delete-kind-cluster

## Setup OLM into the currently selected cluster.
setup-olm:
	@echo "installing OLM $(OLM_VERSION)..."
	@(kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/$(OLM_VERSION)/crds.yaml; \
		kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/$(OLM_VERSION)/olm.yaml; \
		echo -e "\nwaiting for deployment/olm-operator..."; \
		kubectl wait --for=condition=available deployment/olm-operator -n olm --timeout=240s; \
		echo -e "\nwaiting for deployment/catalog-operator..."; \
		kubectl wait --for=condition=available deployment/catalog-operator -n olm --timeout=240s; \
		echo; \
	) 2>&1 | sed 's/^/  /'
.PHONY: setup-olm

## Setup the OpenShift/OKD console into the currently selected cluster.
setup-okd-console:
	@echo "installing OpenShift console :latest..."
	@(kubectl apply -f hack/openshift-console.yaml; \
		echo; \
	) 2>&1 | sed 's/^/  /'
.PHONY: setup-okd-console

## Setup Prometheus Kubernetes stack
setup-monitoring: helm
	@(kubectl create ns monitoring)
	@(helm repo add prometheus-community https://prometheus-community.github.io/helm-charts)
	@(helm repo update)
	@(helm install prometheus prometheus-community/kube-prometheus-stack -n monitoring \
     --set grafana.enabled=false \
     --set kubeStateMetrics.enabled=false \
     --set nodeExporter.enabled=false)

## Loads the OCM API Mock into the currently selected cluster.
prepare-api-mock: \
	load-api-mock \
	config/deploy/api-mock/deployment.yaml
.PHONY: prepare-api-mock

## Loads the Addon Operator Webhook into the currently selected cluster.
prepare-addon-operator-webhook: \
	load-addon-operator-webhook \
	config/deploy/webhook/deployment.yaml
.PHONY: prepare-addon-operator-webhook

## Loads the Addon Operator into the currently selected cluster.
prepare-addon-operator: \
	load-addon-operator \
	config/deploy/deployment.yaml
.PHONY: prepare-addon-operator

## Load Addon Operator images into kind
load-addon-operator: build-image-addon-operator-manager
	@source hack/determine-container-runtime.sh; \
		$$KIND_COMMAND load image-archive \
			.cache/image/addon-operator-manager.tar \
			--name=$(KIND_CLUSTER_NAME);
.PHONY: load-addon-operator

## Load Addon Operator Webhook images into kind
load-addon-operator-webhook: build-image-addon-operator-webhook
	@source hack/determine-container-runtime.sh; \
		$$KIND_COMMAND load image-archive \
			.cache/image/addon-operator-webhook.tar \
			--name=$(KIND_CLUSTER_NAME);
.PHONY: load-addon-operator-webhook

## Load OCM API mock images into kind
load-api-mock: build-image-api-mock
	@source hack/determine-container-runtime.sh; \
		$$KIND_COMMAND load image-archive \
			.cache/image/api-mock.tar \
			--name=$(KIND_CLUSTER_NAME);
.PHONY: load-api-mock

# Template deployment for Addon Operator
config/deploy/deployment.yaml: FORCE yq
	@yq eval '(.spec.template.spec.containers[] | select(.name == "manager")).image = "$(ADDON_OPERATOR_MANAGER_IMAGE)"' \
		config/deploy/deployment.yaml.tpl > config/deploy/deployment.yaml

# Template deployment for OCM API Mock
config/deploy/api-mock/deployment.yaml: FORCE yq
	@yq eval '.spec.template.spec.containers[0].image = "$(API_MOCK_IMAGE)"' \
		config/deploy/api-mock/deployment.yaml.tpl > config/deploy/api-mock/deployment.yaml

# Template deployment for Addon Operator Webhook
config/deploy/webhook/deployment.yaml: FORCE yq
	@yq eval '.spec.template.spec.containers[0].image = "$(ADDON_OPERATOR_WEBHOOK_IMAGE)" | .spec.template.spec.containers[0].ports[0].containerPort = $(WEBHOOK_PORT)' \
		config/deploy/webhook/deployment.yaml.tpl > config/deploy/webhook/deployment.yaml;
	@yq eval '.spec.ports[0].targetPort = $(WEBHOOK_PORT)' \
	config/deploy/webhook/service.yaml.tpl > config/deploy/webhook/service.yaml

## Loads and installs the Addon Operator into the currently selected cluster.
setup-addon-operator: prepare-addon-operator
	@echo "installing Addon Operator $(VERSION)..."
	@(source hack/determine-container-runtime.sh; \
		kubectl apply -f config/deploy; \
		echo -e "\nwaiting for deployment/addon-operator..."; \
		kubectl wait --for=condition=available deployment/addon-operator -n addon-operator --timeout=240s; \
		echo; \
	) 2>&1 | sed 's/^/  /'
ifneq ($(ENABLE_WEBHOOK), "false")
	@make prepare-addon-operator-webhook
endif
ifneq ($(ENABLE_API_MOCK), "false")
	@make prepare-api-mock
endif
ifeq ($(ENABLE_MONITORING), "true")
	@make setup-monitoring
	@(source hack/determine-container-runtime.sh; \
		kubectl apply -f config/deploy/monitoring; \
		echo; \
	) 2>&1 | sed 's/^/  /'
endif
.PHONY: setup-addon-operator

## Installs Addon Operator CRDs in to the currently selected cluster.
setup-addon-operator-crds: generate
	@for crd in $(wildcard config/deploy/*.openshift.io_*.yaml); do \
		kubectl apply -f $$crd; \
	done
.PHONY: setup-addon-operator-crds

# ------------------
##@ Container Images
# ------------------

## Build all images.
build-images:
	./mage build:buildimages
.PHONY: build-images

## Build and push all images.
push-images:
	./mage build:pushimages
.PHONY: push-images

# App Interface specific push-images target, to run within a docker container.
app-interface-push-images:
	@echo "-------------------------------------------------"
	@echo "running in app-interface-push-images container..."
	@echo "-------------------------------------------------"
	$(eval IMAGE_NAME := app-interface-push-images)
	@(source hack/determine-container-runtime.sh; \
		$$CONTAINER_COMMAND build -t "${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}" -f "config/docker/${IMAGE_NAME}.Dockerfile" --pull .; \
		$$CONTAINER_COMMAND run --rm \
			--privileged \
			-e JENKINS_HOME=${JENKINS_HOME} \
			-e QUAY_USER=${QUAY_USER} \
			-e QUAY_TOKEN=${QUAY_TOKEN} \
			"${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}" \
			make push-images; \
	echo) 2>&1 | sed 's/^/  /'
.PHONY: app-interface-push-images

## openshift release openshift-ci operator
openshift-ci-test-build: \
	clean-config-openshift
	@ADDON_OPERATOR_MANAGER_IMAGE=quay.io/openshift/addon-operator:latest ADDON_OPERATOR_WEBHOOK_IMAGE=quay.io/openshift/addon-operator-webhook:latest ./mage build:TemplateAddonOperatorCSV
	$(eval IMAGE_NAME := addon-operator-bundle)
	@echo "preparing files for config/openshift ${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}..."
	@mkdir -p "config/openshift/manifests";
	@mkdir -p "config/openshift/metadata";
	@cp "config/docker/${IMAGE_NAME}.Dockerfile" "config/openshift/${IMAGE_NAME}.Dockerfile";
	@cp "config/olm/annotations.yaml" "config/openshift/metadata";
	@cp "config/olm/metrics.service.yaml" "config/openshift/manifests/metrics.service.yaml";
	@cp "config/olm/addon-operator.csv.yaml" "config/openshift/manifests/addon-operator.csv.yaml";
	@tail -n"+3" "config/deploy/addons.managed.openshift.io_addons.yaml" > "config/openshift/manifests/addons.crd.yaml";
	@tail -n"+3" "config/deploy/addons.managed.openshift.io_addonoperators.yaml" > "config/openshift/manifests/addonoperators.crd.yaml";
	@tail -n"+3" "config/deploy/addons.managed.openshift.io_addoninstances.yaml" > "config/openshift/manifests/addoninstances.crd.yaml";

.SECONDEXPANSION:

## Builds config/docker/%.Dockerfile using a binary build from cmd/%.
build-image-%:
	./mage build:imagebuild $*

## Build and push config/docker/%.Dockerfile using a binary build from cmd/%.
push-image-%:
	./mage build:imagepush $*

# cleans the config/openshift folder for addon-operator-bundle openshift test folder
clean-config-openshift:
	@rm -rf "config/openshift/*"
