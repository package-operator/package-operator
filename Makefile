SHELL=/bin/bash
.SHELLFLAGS=-euo pipefail -c

# Dependency Versions
CONTROLLER_GEN_VERSION:=v0.5.0
OLM_VERSION:=v0.17.0
KIND_VERSION:=v0.10.0
YQ_VERSION:=v4@v4.7.0
GOIMPORTS_VERSION:=v0.1.0
GOLANGCI_LINT_VERSION:=v1.39.0
OPM_VERSION:=v1.17.2

# Build Flags
export CGO_ENABLED:=0
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
SHORT_SHA=$(shell git rev-parse --short HEAD)
VERSION?=$(shell echo ${BRANCH} | tr / -)-${SHORT_SHA}
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
export PATH:=$(DEPENDENCY_BIN):$(PATH)

# Config
KIND_KUBECONFIG_DIR:=.cache/e2e
KIND_KUBECONFIG:=$(KIND_KUBECONFIG_DIR)/kubeconfig
export KUBECONFIG?=$(abspath $(KIND_KUBECONFIG))
export GOLANGCI_LINT_CACHE=$(abspath .cache/golangci-lint)
export SKIP_TEARDOWN?=
KIND_CLUSTER_NAME:="addon-operator" # name of the kind cluster for local development.

# Container
IMAGE_ORG?=quay.io/app-sre
ADDON_OPERATOR_MANAGER_IMAGE?=$(IMAGE_ORG)/addon-operator-manager:$(VERSION)

# COLORS
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET  := $(shell tput -Txterm sgr0)

# ---------
##@ General
# ---------

# Default build target - must be first!
all: \
	bin/linux_amd64/addon-operator-manager

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

# go-get-tool will 'go get' any package $1 if file $2 does not exist.
define go-get-tool
@[ -f "$(2)" ] || { \
	TMP_DIR=$$(mktemp -d); \
	cd $$TMP_DIR; \
	go mod init tmp; \
	echo "Downloading $(1) to $(DEPENDENCIES)/bin"; \
	GOBIN="$(DEPENDENCY_BIN)" go get "$(1)"; \
	rm -rf $$TMP_DIR; \
	mkdir -p "$(dir $(2))"; \
	touch "$(2)"; \
}
endef

KIND:=$(DEPENDENCY_VERSIONS)/kind/$(KIND_VERSION)
$(KIND):
	@$(call go-get-tool,sigs.k8s.io/kind@$(KIND_VERSION),$(KIND))

CONTROLLER_GEN:=$(DEPENDENCY_VERSIONS)/controller-gen/$(CONTROLLER_GEN_VERSION)
$(CONTROLLER_GEN):
	@$(call go-get-tool,sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION),$(CONTROLLER_GEN))

YQ:=$(DEPENDENCY_VERSIONS)/yq/$(YQ_VERSION)
$(YQ):
	@$(call go-get-tool,github.com/mikefarah/yq/$(YQ_VERSION),$(YQ))

GOIMPORTS:=$(DEPENDENCY_VERSIONS)/goimports/$(GOIMPORTS_VERSION)
$(GOIMPORTS):
	@$(call go-get-tool,golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION),$(GOIMPORTS))

# Setup goimports.
# alias for goimports to use from `ensure-and-run-goimports.sh` via pre-commit.
goimports: $(GOIMPORTS)
.PHONY: goimports

GOLANGCI_LINT:=$(DEPENDENCY_VERSIONS)/golangci-lint/$(GOLANGCI_LINT_VERSION)
$(GOLANGCI_LINT):
	@$(call go-get-tool,github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION),$(GOLANGCI_LINT))

OPM:=$(DEPENDENCY_VERSIONS)/opm/$(OPM_VERSION)
$(OPM):
	@echo "installing opm $(OPM_VERSION)..."
	$(eval OPM_TMP := $(shell mktemp -d))
	@(cd "$(OPM_TMP)"; \
		curl -L --fail \
		https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/linux-amd64-opm -o opm; \
		chmod +x opm; \
		mv opm $(GOBIN); \
	) 2>&1 | sed 's/^/  /'
	@rm -rf "$(OPM_TMP)" "$(dir $(OPM))" \
		&& mkdir -p "$(dir $(OPM))" \
		&& touch "$(OPM)" \
		&& echo

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

## Runs the E2E testsuite against the current $KUBECONFIG cluster.
test-e2e: config/deploy/deployment.yaml
	@echo "running e2e tests..."
	@go test -v -count=1 ./e2e/...
.PHONY: test-e2e

## Runs the E2E testsuite against the current $KUBECONFIG cluster. Skips operator setup and teardown.
test-e2e-short: config/deploy/deployment.yaml
	@echo "running [short] e2e tests..."
	@go test -v -count=1 -short ./e2e/...

# make sure that we install our components into the kind cluster and disregard normal $KUBECONFIG
test-e2e-local: export KUBECONFIG=$(abspath $(KIND_KUBECONFIG))
## Setup a local dev environment and execute the full e2e testsuite against it.
test-e2e-local: | dev-setup load-addon-operator test-e2e
.PHONY: test-e2e-local

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
.PHONY: run-addon-operator-manager

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

## Setup a local env for integration/e2e test development. (Kind, OLM, OKD Console, Addon Operator). Use with test-e2e-short.
test-setup: | \
	dev-setup \
	setup-addon-operator
.PHONY: test-setup

## Creates an empty kind cluster to be used for local development.
create-kind-cluster: $(KIND)
	@echo "creating kind cluster addon-operator-e2e..."
	@mkdir -p .cache/e2e
	@(source hack/determine-container-runtime.sh; \
		mkdir -p $(KIND_KUBECONFIG_DIR); \
		$$KIND_COMMAND create cluster \
			--kubeconfig=$(KIND_KUBECONFIG) \
			--name=$(KIND_CLUSTER_NAME); \
		if [[ ! -O "$(KIND_KUBECONFIG)" ]]; then \
			sudo chown $$USER: "$(KIND_KUBECONFIG)"; \
		fi; \
		echo; \
	) 2>&1 | sed 's/^/  /'
.PHONY: create-kind-cluster

## Deletes the previously created kind cluster.
delete-kind-cluster: $(KIND)
	@echo "deleting kind cluster addon-operator-e2e..."
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

# Template deployment
config/deploy/deployment.yaml: FORCE $(YQ)
	@yq eval '.spec.template.spec.containers[0].image = "$(ADDON_OPERATOR_MANAGER_IMAGE)"' \
		config/deploy/deployment.yaml.tpl > config/deploy/deployment.yaml

## Loads and installs the Addon Operator into the currently selected cluster.
setup-addon-operator: $(YQ) load-addon-operator config/deploy/deployment.yaml
	@echo "installing Addon Operator $(VERSION)..."
	@(source hack/determine-container-runtime.sh; \
		kubectl apply -f config/deploy; \
		echo -e "\nwaiting for deployment/addon-operator..."; \
		kubectl wait --for=condition=available deployment/addon-operator -n addon-operator --timeout=240s; \
		echo; \
	) 2>&1 | sed 's/^/  /'
.PHONY: setup-addon-operator

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
	build-image-addon-operator-manager
.PHONY: build-images

## Build and push all images.
push-images: \
	push-image-addon-operator-manager
.PHONY: push-images

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
push-image-%: build-image-$$*
	@echo "pushing image ${IMAGE_ORG}/$*:${VERSION}..."
	@(source hack/determine-container-runtime.sh; \
		$$CONTAINER_COMMAND push "${IMAGE_ORG}/$*:${VERSION}"; \
		echo pushed "${IMAGE_ORG}/$*:${VERSION}"; \
		echo; \
	) 2>&1 | sed 's/^/  /'
