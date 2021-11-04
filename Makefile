SHELL=/bin/bash
.SHELLFLAGS=-euo pipefail -c

# Dependency Versions
CONTROLLER_GEN_VERSION:=v0.6.2
OLM_VERSION:=v0.18.3
KIND_VERSION:=v0.11.1
YQ_VERSION:=v4@v4.12.0
GOIMPORTS_VERSION:=v0.1.5
GOLANGCI_LINT_VERSION:=v1.42.0
OPM_VERSION:=v1.18.0

# Build Flags
export CGO_ENABLED:=0
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
SHORT_SHA=$(shell git rev-parse --short HEAD)
VERSION?=${SHORT_SHA}
BUILD_DATE=$(shell date +%s)
MODULE:=github.com/openshift/addon-operator
LD_FLAGS=-X $(MODULE)/internal/version.Version=$(VERSION) \
			-X $(MODULE)/internal/version.Branch=$(BRANCH) \
			-X $(MODULE)/internal/version.Commit=$(SHORT_SHA) \
			-X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

UNAME_OS:=$(shell uname -s)
UNAME_ARCH:=$(shell uname -m)

# PATH/Bin
DEPENDENCIES:=.cache/dependencies
DEPENDENCY_BIN:=$(abspath $(DEPENDENCIES)/bin)
DEPENDENCY_VERSIONS:=$(abspath $(DEPENDENCIES)/$(UNAME_OS)/$(UNAME_ARCH)/versions)
# explicitly add go path for app-interface
export PATH:=/opt/go/1.16.9/bin:$(DEPENDENCY_BIN):$(PATH)

# Config
KIND_KUBECONFIG_DIR:=.cache/integration
KIND_KUBECONFIG:=$(KIND_KUBECONFIG_DIR)/kubeconfig
export KUBECONFIG?=$(abspath $(KIND_KUBECONFIG))
export GOLANGCI_LINT_CACHE=$(abspath .cache/golangci-lint)
export SKIP_TEARDOWN?=
KIND_CLUSTER_NAME:="addon-operator" # name of the kind cluster for local development.
ENABLE_WEBHOOK?="false"
WEBHOOK_PORT?=8080

# Container
IMAGE_ORG?=quay.io/app-sre
ADDON_OPERATOR_MANAGER_IMAGE?=$(IMAGE_ORG)/addon-operator-manager:$(VERSION)
ADDON_OPERATOR_WEBHOOK_IMAGE?=$(IMAGE_ORG)/addon-operator-webhook:$(VERSION)

ifdef JENKINS_HOME
export DOCKER_CONF:=$(abspath .docker)
endif


# COLORS
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET  := $(shell tput -Txterm sgr0)

# ---------
##@ General
# ---------

# Default build target - must be first!
all: \
	bin/linux_amd64/addon-operator-manager \
	bin/linux_amd64/addon-operator-webhook

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

## Forces GOOS=linux GOARCH=amd64. For bin/%.
bin/linux_amd64/%: GOARGS = GOOS=linux GOARCH=amd64

## Builds binaries from cmd/%.
bin/%: generate FORCE
	$(eval COMPONENT=$(shell basename $*))
	@echo -e -n "compiling cmd/$(COMPONENT)...\n  "
	$(GOARGS) go build -ldflags "-w $(LD_FLAGS)" -o bin/$* cmd/$(COMPONENT)/main.go
	@echo

# empty force target to ensure a target always executes.
FORCE:

# ----------------------------
# Dependencies (project local)
# ----------------------------

# go-install-tool will 'go install' any package $1 if file $2 does not exist.
define go-install-tool
@[ -f "$(2)" ] || { \
	TMP_DIR=$$(mktemp -d); \
	cd $$TMP_DIR; \
	go mod init tmp; \
	echo "Downloading $(1) to $(DEPENDENCIES)/bin"; \
	GOBIN="$(DEPENDENCY_BIN)" go install -mod=readonly "$(1)"; \
	rm -rf $$TMP_DIR; \
	mkdir -p "$(dir $(2))"; \
	touch "$(2)"; \
}
endef

KIND:=$(DEPENDENCY_VERSIONS)/kind/$(KIND_VERSION)
$(KIND):
	@$(call go-install-tool,sigs.k8s.io/kind@$(KIND_VERSION),$(KIND))
	@(which kind; kind version) | sed 's/^/  /'

CONTROLLER_GEN:=$(DEPENDENCY_VERSIONS)/controller-gen/$(CONTROLLER_GEN_VERSION)
$(CONTROLLER_GEN):
	@$(call go-install-tool,sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION),$(CONTROLLER_GEN))
	@(echo; which controller-gen; controller-gen --version; echo) | sed 's/^/  /'

YQ:=$(DEPENDENCY_VERSIONS)/yq/$(YQ_VERSION)
$(YQ):
	@$(call go-install-tool,github.com/mikefarah/yq/$(YQ_VERSION),$(YQ))
	@(which yq; yq --version) | sed 's/^/  /'

GOIMPORTS:=$(DEPENDENCY_VERSIONS)/goimports/$(GOIMPORTS_VERSION)
$(GOIMPORTS):
	@$(call go-install-tool,golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION),$(GOIMPORTS))
	# goimports doesn't have a version flag
	@(echo; which goimports; echo) | sed 's/^/  /'

# Setup goimports.
# alias for goimports to use from `ensure-and-run-goimports.sh` via pre-commit.
goimports: $(GOIMPORTS)
.PHONY: goimports

GOLANGCI_LINT:=$(DEPENDENCY_VERSIONS)/golangci-lint/$(GOLANGCI_LINT_VERSION)
$(GOLANGCI_LINT):
	@$(call go-install-tool,github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION),$(GOLANGCI_LINT))
	@(echo; which golangci-lint; golangci-lint --version; echo) | sed 's/^/  /'

# Setup golangci-lint.
# alias for golangci-lint to use from `ensure-and-run-golangci-lint.sh` via pre-commit.
golangci-lint: $(GOLANGCI_LINT)
.PHONY: golangci-lint

OPM:=$(DEPENDENCY_VERSIONS)/opm/$(OPM_VERSION)
$(OPM):
	@echo "installing opm $(OPM_VERSION)..."
	$(eval OPM_TMP := $(shell mktemp -d))
	@(cd "$(OPM_TMP)"; \
		curl -L --fail \
		https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/linux-amd64-opm -o opm; \
		chmod +x opm; \
		mv opm $(DEPENDENCY_BIN); \
	) 2>&1 | sed 's/^/  /'
	@rm -rf "$(OPM_TMP)" "$(dir $(OPM))" \
		&& mkdir -p "$(dir $(OPM))" \
		&& touch "$(OPM)" \
		&& echo
	@(echo; which opm; opm version; echo) | sed 's/^/  /'

# ------------
##@ Generators
# ------------

## Generate deepcopy code and kubernetes manifests.
generate: $(CONTROLLER_GEN)
	@echo "generating kubernetes manifests..."
	@controller-gen crd:crdVersions=v1 \
		rbac:roleName=addon-operator-manager \
		paths="./..." \
		output:crd:artifacts:config=config/deploy 2>&1 | sed 's/^/  /'
	@echo
	@echo "generating code..."
	@controller-gen object paths=./apis/... 2>&1 | sed 's/^/  /'
	@echo
	@echo "patching generated code to stay go 1.16 output compliant - https://golang.org/doc/go1.17#gofmt"
	@echo "TODO: remove this when we move to go 1.17"
	@echo "otherwise our ci will fail because of changed files"
	@echo "this removes the line '//go:build !ignore_autogenerated'"
	@find ./apis -name 'zz_generated.deepcopy.go' -exec sed -i '/\/\/go:build !ignore_autogenerated/d' {} \;
	@echo
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
lint: generate $(GOLANGCI_LINT)
	go fmt ./...
	@hack/validate-directory-clean.sh
	golangci-lint run ./... --deadline=15m
.PHONY: lint

## Runs code-generators and unittests.
test-unit: generate
	CGO_ENABLED=1 go test -race -v ./internal/... ./cmd/...
.PHONY: test-unit

## Runs the Integration testsuite against the current $KUBECONFIG cluster
test-integration: config/deploy/deployment.yaml \
	config/deploy/webhook/deployment.yaml
	@echo "running integration tests..."
	@go test -v -count=1 -timeout=20m ./integration/...
.PHONY: test-integration

# legacy alias for CI/CD
test-e2e: ENABLE_WEBHOOK?=true
test-e2e: test-integration
.PHONY: test-e2e

## Runs the Integration testsuite against the current $KUBECONFIG cluster. Skips operator setup and teardown.
test-integration-short: config/deploy/deployment.yaml \
	config/deploy/webhook/deployment.yaml
	@echo "running [short] integration tests..."
	@go test -v -count=1 -short ./integration/...

# make sure that we install our components into the kind cluster and disregard normal $KUBECONFIG
test-integration-local: export KUBECONFIG=$(abspath $(KIND_KUBECONFIG))
## Setup a local dev environment and execute the full integration testsuite against it.
test-integration-local: | dev-setup load-addon-operator \
	load-addon-operator-webhook test-integration
.PHONY: test-integration-local

# -------------------------
##@ Development Environment
# -------------------------

## Installs all project dependencies into $(PWD)/.cache/bin
dependencies: \
	$(KIND) \
	$(CONTROLLER_GEN) \
	$(YQ) \
	$(GOIMPORTS) \
	$(GOLANGCI_LINT)
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
create-kind-cluster: $(KIND)
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
.PHONY: create-kind-cluster

## Deletes the previously created kind cluster.
delete-kind-cluster: $(KIND)
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

# Template deployment for Addon Operator
config/deploy/deployment.yaml: FORCE $(YQ)
	@yq eval '.spec.template.spec.containers[0].image = "$(ADDON_OPERATOR_MANAGER_IMAGE)"' \
		config/deploy/deployment.yaml.tpl > config/deploy/deployment.yaml

# Template deployment for Addon Operator Webhook
config/deploy/webhook/deployment.yaml: FORCE $(YQ)
	@yq eval '.spec.template.spec.containers[0].image = "$(ADDON_OPERATOR_WEBHOOK_IMAGE)" | .spec.template.spec.containers[0].ports[0].containerPort = $(WEBHOOK_PORT)' \
		config/deploy/webhook/deployment.yaml.tpl > config/deploy/webhook/deployment.yaml;
	@yq eval '.spec.ports[0].targetPort = $(WEBHOOK_PORT)' \
	config/deploy/webhook/service.yaml.tpl > config/deploy/webhook/service.yaml


## Loads and installs the Addon Operator into the currently selected cluster.
setup-addon-operator: $(YQ) load-addon-operator config/deploy/deployment.yaml
	@echo "installing Addon Operator $(VERSION)..."
	@(source hack/determine-container-runtime.sh; \
		kubectl apply -f config/deploy; \
		echo -e "\nwaiting for deployment/addon-operator..."; \
		kubectl wait --for=condition=available deployment/addon-operator -n addon-operator --timeout=240s; \
		echo; \
	) 2>&1 | sed 's/^/  /'
ifneq ($(ENABLE_WEBHOOK), "false")
	@make setup-addon-operator-webhook
endif
.PHONY: setup-addon-operator


## Loads and installs the Addon Operator Webhook into the currently selected cluster.
setup-addon-operator-webhook: $(YQ) load-addon-operator-webhook \
	config/deploy/webhook/deployment.yaml
	@echo "setting up TLS cert..."
	@kubectl apply -f \
		config/deploy/webhook/00-tls-secret.yaml
	@echo "installing Addon Operator $(VERSION)..."
	@(source hack/determine-container-runtime.sh; \
		kubectl apply -f config/deploy/webhook/deployment.yaml; \
		echo -e "\nwaiting for deployment/addon-operator-webhook..."; \
		kubectl wait --for=condition=available deployment/addon-operator-webhook -n addon-operator --timeout=240s; \
		kubectl apply -f config/deploy/webhook/service.yaml; \
		kubectl apply -f config/deploy/webhook/validatingwebhookconfig.yaml; \
		echo; \
	) 2>&1 | sed 's/^/  /'

## Installs Addon Operator CRDs in to the currently selected cluster.
setup-addon-operator-crds: generate
	@for crd in $(wildcard config/deploy/*.openshift.io_*.yaml); do \
		kubectl apply -f $$crd; \
	done
.PHONY: setup-addon-operator-crds

# ---
# OLM
# ---

# Template Cluster Service Version / CSV
# By setting the container image to deploy.
config/olm/addon-operator.csv.yaml: FORCE $(YQ)
	@yq eval '.spec.install.spec.deployments[0].spec.template.spec.containers[0].image = "$(ADDON_OPERATOR_MANAGER_IMAGE)" | .metadata.annotations.containerImage = "$(ADDON_OPERATOR_MANAGER_IMAGE)"' \
	config/olm/addon-operator.csv.tpl.yaml > config/olm/addon-operator.csv.yaml

# Bundle image contains the manifests and CSV for a single version of this operator.
# The first few lines of the CRD file need to be removed:
# https://github.com/operator-framework/operator-registry/issues/222
build-image-addon-operator-bundle: \
	clean-image-cache-addon-operator-bundle \
	config/olm/addon-operator.csv.yaml
	$(eval IMAGE_NAME := addon-operator-bundle)
	@echo "building image ${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}..."
	@(source hack/determine-container-runtime.sh; \
		mkdir -p ".cache/image/${IMAGE_NAME}/manifests"; \
		mkdir -p ".cache/image/${IMAGE_NAME}/metadata"; \
		cp -a "config/olm/addon-operator.csv.yaml" ".cache/image/${IMAGE_NAME}/manifests"; \
		cp -a "config/olm/annotations.yaml" ".cache/image/${IMAGE_NAME}/metadata"; \
		cp -a "config/docker/${IMAGE_NAME}.Dockerfile" ".cache/image/${IMAGE_NAME}/Dockerfile"; \
		tail -n"+3" "config/deploy/addons.managed.openshift.io_addons.yaml" > ".cache/image/${IMAGE_NAME}/manifests/addons.crd.yaml"; \
		tail -n"+3" "config/deploy/addons.managed.openshift.io_addonoperators.yaml" > ".cache/image/${IMAGE_NAME}/manifests/addonoperatorss.crd.yaml"; \
		$$CONTAINER_COMMAND build -t "${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}" ".cache/image/${IMAGE_NAME}"; \
		$$CONTAINER_COMMAND image save -o ".cache/image/${IMAGE_NAME}.tar" "${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}"; \
		echo) 2>&1 | sed 's/^/  /'
.PHONY: build-image-addon-operator-bundle

# Index image contains a list of bundle images for use in a CatalogSource.
# Warning!
# The bundle image needs to be pushed so the opm CLI can create the index image.
build-image-addon-operator-index: $(OPM) \
	clean-image-cache-addon-operator-index \
	| build-image-addon-operator-bundle \
	push-image-addon-operator-bundle
	$(eval IMAGE_NAME := addon-operator-index)
	@echo "building image ${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}..."
	@(source hack/determine-container-runtime.sh; \
		opm index add --container-tool $$CONTAINER_COMMAND \
		--bundles ${IMAGE_ORG}/addon-operator-bundle:${VERSION} \
		--tag ${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}; \
		$$CONTAINER_COMMAND image save -o ".cache/image/${IMAGE_NAME}.tar" "${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}"; \
		echo) 2>&1 | sed 's/^/  /'
.PHONY: build-image-addon-operator-index

# ------------------
##@ Container Images
# ------------------

## Build all images.
build-images: \
	build-image-addon-operator-manager \
	build-image-addon-operator-webhook
.PHONY: build-images

## Build and push all images.
push-images: \
	push-image-addon-operator-manager \
	push-image-addon-operator-webhook \
	push-image-addon-operator-index
.PHONY: push-images

docker-login:
ifdef JENKINS_HOME
	@echo running in Jenkins, calling docker login
	@mkdir -p "${DOCKER_CONF}"
	@docker --config="${DOCKER_CONF}" login -u="${QUAY_USER}" -p="${QUAY_TOKEN}" quay.io
endif

# App Interface specific push-images target, to run within a docker container.
app-interface-push-images:
	@echo "-------------------------------------------------"
	@echo "running in app-interface-push-images container..."
	@echo "-------------------------------------------------"
	$(eval IMAGE_NAME := app-interface-push-images)
	@(source hack/determine-container-runtime.sh; \
		$$CONTAINER_COMMAND build -t "${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}" -f "config/docker/${IMAGE_NAME}.Dockerfile" .; \
		$$CONTAINER_COMMAND run --rm \
			-v /var/run/docker.sock:/var/run/docker.sock \
			-e JENKINS_HOME=${JENKINS_HOME} \
			-e QUAY_USER=${QUAY_USER} \
			-e QUAY_TOKEN=${QUAY_TOKEN} \
			"${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}" \
			make push-images; \
	echo) 2>&1 | sed 's/^/  /'
.PHONY: push-images-in-container

.SECONDEXPANSION:
# cleans the built image .tar and image build directory
clean-image-cache-%:
	@rm -rf ".cache/image/$*" ".cache/image/$*.tar"
	@mkdir -p ".cache/image/$*"

## Builds config/docker/%.Dockerfile using a binary build from cmd/%.
build-image-%: bin/linux_amd64/$$*
	@echo "building image ${IMAGE_ORG}/$*:${VERSION}..."
	@(source hack/determine-container-runtime.sh; \
		rm -rf ".cache/image/$*" ".cache/image/$*.tar"; \
		mkdir -p ".cache/image/$*"; \
		cp -a "bin/linux_amd64/$*" ".cache/image/$*"; \
		cp -a "config/docker/$*.Dockerfile" ".cache/image/$*/Dockerfile"; \
		cp -a "config/docker/passwd" ".cache/image/$*/passwd"; \
		$$CONTAINER_COMMAND build -t "${IMAGE_ORG}/$*:${VERSION}" ".cache/image/$*"; \
		$$CONTAINER_COMMAND image save -o ".cache/image/$*.tar" "${IMAGE_ORG}/$*:${VERSION}"; \
		echo; \
	) 2>&1 | sed 's/^/  /'

## Build and push config/docker/%.Dockerfile using a binary build from cmd/%.
push-image-%: docker-login build-image-$$*
	@echo "pushing image ${IMAGE_ORG}/$*:${VERSION}..."
	@(source hack/determine-container-runtime.sh; \
		CONFIG_FLAG=""; \
		if [[ ! -z "$${DOCKER_CONF+x}" ]]; then \
			CONFIG_FLAG="--config $$DOCKER_CONF"; \
		fi; \
		$$CONTAINER_COMMAND $$CONFIG_FLAG push "${IMAGE_ORG}/$*:${VERSION}"; \
		echo pushed "${IMAGE_ORG}/$*:${VERSION}"; \
		echo; \
	) 2>&1 | sed 's/^/  /'
