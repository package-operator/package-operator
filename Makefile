# TODOs
# relocate IMAGE_ORG

IMAGE_ORG?=quay.io/openshift
MODULE:=github.com/openshift/addon-operator
KIND_KUBECONFIG:=bin/e2e/kubeconfig

# Dependency Versions
CONTROLLER_GEN_VERSION:=v0.5.0
OLM_VERSION:=v0.17.0
KIND_VERSION:=v0.10.0
YQ_VERSION:=v4@v4.7.0
GOIMPORTS_VERSION:=v0.1.0

SHELL=/bin/bash
.SHELLFLAGS=-euo pipefail -c

# Build Flags
export CGO_ENABLED:=0
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
SHORT_SHA=$(shell git rev-parse --short HEAD)
VERSION?=${BRANCH}-${SHORT_SHA}
BUILD_DATE=$(shell date +%s)
LD_FLAGS=-X $(MODULE)/internal/version.Version=$(VERSION) \
			-X $(MODULE)/internal/version.Branch=$(BRANCH) \
			-X $(MODULE)/internal/version.Commit=$(SHORT_SHA) \
			-X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

UNAME_OS:=$(shell uname -s)
UNAME_ARCH:=$(shell uname -m)

# PATH/Bin
DEPENDENCIES:=bin/dependencies/$(UNAME_OS)/$(UNAME_ARCH)
export GOBIN?=$(abspath bin/dependencies/bin)
export PATH:=$(GOBIN):$(PATH)

# -------
# Compile
# -------

all: \
	bin/linux_amd64/addon-operator-manager

bin/linux_amd64/%: GOARGS = GOOS=linux GOARCH=amd64

bin/%: generate manifests FORCE
	$(eval COMPONENT=$(shell basename $*))
	@echo -e -n "compiling cmd/$(COMPONENT)...\n  "
	$(GOARGS) go build -ldflags "-w $(LD_FLAGS)" -o bin/$* cmd/$(COMPONENT)/main.go
	@echo

FORCE:

clean:
	rm -rf bin/$*
.PHONY: clean

# ------------
# Dependencies
# ------------

# setup kind
KIND:=$(DEPENDENCIES)/kind/$(KIND_VERSION)
$(KIND):
	@echo "installing kind $(KIND_VERSION)..."
	$(eval KIND_TMP := $(shell mktemp -d))
	@(cd "$(KIND_TMP)" \
		&& go mod init tmp \
		&& go get "sigs.k8s.io/kind@$(KIND_VERSION)" \
	) 2>&1 | sed 's/^/  /'
	@rm -rf "$(KIND_TMP)" "$(dir $(KIND))" \
		&& mkdir -p "$(dir $(KIND))" \
		&& touch "$(KIND)" \
		&& echo

# setup controller-gen
CONTROLLER_GEN:=$(DEPENDENCIES)/controller-gen/$(CONTROLLER_GEN_VERSION)
$(CONTROLLER_GEN):
	@echo "installing controller-gen $(CONTROLLER_GEN_VERSION)..."
	$(eval CONTROLLER_GEN_TMP := $(shell mktemp -d))
	@(cd "$(CONTROLLER_GEN_TMP)" \
		&& go mod init tmp \
		&& go get "sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)" \
	) 2>&1 | sed 's/^/  /'
	@rm -rf "$(CONTROLLER_GEN_TMP)" "$(dir $(CONTROLLER_GEN))" \
		&& mkdir -p "$(dir $(CONTROLLER_GEN))" \
		&& touch "$(CONTROLLER_GEN)" \
		&& echo

# setup yq
YQ:=$(DEPENDENCIES)/yq/$(YQ_VERSION)
$(YQ):
	@echo "installing yq $(YQ_VERSION)..."
	$(eval YQ_TMP := $(shell mktemp -d))
	@(cd "$(YQ_TMP)" \
		&& go mod init tmp \
		&& go get "github.com/mikefarah/yq/$(YQ_VERSION)" \
	) 2>&1 | sed 's/^/  /'
	@rm -rf "$(YQ_TMP)" "$(dir $(YQ))" \
		&& mkdir -p "$(dir $(YQ))" \
		&& touch "$(YQ)" \
		&& echo

# setup goimports
GOIMPORTS:=$(DEPENDENCIES)/GOIMPORTS/$(GOIMPORTS_VERSION)
$(GOIMPORTS):
	@echo "installing GOIMPORTS $(GOIMPORTS_VERSION)..."
	$(eval GOIMPORTS_TMP := $(shell mktemp -d))
	@(cd "$(GOIMPORTS_TMP)" \
		&& go mod init tmp \
		&& go get "golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)" \
	) 2>&1 | sed 's/^/  /'
	@rm -rf "$(GOIMPORTS_TMP)" "$(dir $(GOIMPORTS))" \
		&& mkdir -p "$(dir $(GOIMPORTS))" \
		&& touch "$(GOIMPORTS)" \
		&& echo

setup-dependencies: \
	$(KIND) \
	$(CONTROLLER_GEN) \
	$(YQ) \
	$(GOIMPORTS)

# ----------
# Deployment
# ----------

# Run against the configured Kubernetes cluster in ~/.kube/config or $KUBECONFIG
run: generate fmt vet manifests
	go run -ldflags "-w $(LD_FLAGS)" \
		./cmd/addon-operator-manager/main.go \
			-pprof-addr="127.0.0.1:8065"
.PHONY: run

# ----------
# Generators
# ----------

# Generate manifests e.g. CRD, RBAC etc.
manifests: $(CONTROLLER_GEN)
	@echo "generating kubernetes manifests..."
	@controller-gen crd:crdVersions=v1 \
		rbac:roleName=addon-operator-manager \
		paths="./..." \
		output:crd:artifacts:config=config/deploy 2>&1 | sed 's/^/  /'
	@echo

# Generate code
generate: $(CONTROLLER_GEN)
	@echo "generating code..."
	@controller-gen object paths=./apis/... 2>&1 | sed 's/^/  /'
	@echo

# Makes sandwich
# https://xkcd.com/149/
sandwich:
ifneq ($(shell id -u), 0)
	@echo "What? Make it yourself."
else
	@echo "Okay."
endif

# -------------------
# Testing and Linting
# -------------------

test: generate fmt vet manifests
	CGO_ENABLED=1 go test -race -v ./internal/... ./cmd/...
.PHONY: test

ci-test: test
	hack/validate-directory-clean.sh
.PHONY: ci-test

e2e-test:
	@echo "running e2e tests..."
	@export KUBECONFIG=$(abspath $(KIND_KUBECONFIG)) \
		&& kubectl get pod -A \
		&& echo \
		&& go test -v ./e2e/...
.PHONY: e2e-test

e2e: | setup-e2e-kind e2e-test
.PHONY: e2e

fmt:
	go fmt ./...
.PHONY: fmt

vet:
	go vet ./...
.PHONY: vet

pre-commit-install:
	@echo "installing pre-commit hooks using https://pre-commit.com/"
	@pre-commit install
.PHONY: pre-commit-install

create-kind-cluster: $(KIND)
	@echo "creating kind cluster addon-operator-e2e..."
	@mkdir -p bin/e2e
	@(source hack/determine-container-runtime.sh \
		&& $$KIND_COMMAND create cluster \
			--kubeconfig=$(KIND_KUBECONFIG) \
			--name="addon-operator-e2e" \
		&& sudo chown $$USER: $(KIND_KUBECONFIG) \
		&& echo) 2>&1 | sed 's/^/  /'
.PHONY: create-kind-cluster

delete-kind-cluster: $(KIND)
	@echo "deleting kind cluster addon-operator-e2e..."
	@(source hack/determine-container-runtime.sh \
		&& $$KIND_COMMAND delete cluster \
			--kubeconfig="$(KIND_KUBECONFIG)" \
			--name "addon-operator-e2e" \
		&& rm -rf "$(KIND_KUBECONFIG)" \
		&& echo) 2>&1 | sed 's/^/  /'
.PHONY: delete-kind-cluster

setup-e2e-kind: | \
	create-kind-cluster \
	apply-olm \
	apply-openshift-console \
	apply-ao

apply-olm:
	@echo "installing OLM $(OLM_VERSION)..."
	@(export KUBECONFIG=$(KIND_KUBECONFIG) \
		&& kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/$(OLM_VERSION)/crds.yaml \
		&& kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/$(OLM_VERSION)/olm.yaml \
		&& echo -e "\nwaiting for deployment/olm-operator..." \
		&& kubectl wait --for=condition=available deployment/olm-operator -n olm --timeout=240s \
		&& echo -e "\nwaiting for deployment/catalog-operator..." \
		&& kubectl wait --for=condition=available deployment/catalog-operator -n olm --timeout=240s \
		&& echo) 2>&1 | sed 's/^/  /'
.PHONY: apply-olm

apply-openshift-console:
	@echo "installing OpenShift console :latest..."
	@(export KUBECONFIG=$(KIND_KUBECONFIG) \
		&& kubectl apply -f hack/openshift-console.yaml \
		&& echo) 2>&1 | sed 's/^/  /'
.PHONY: apply-openshift-console

apply-ao: $(YQ) build-image-addon-operator-manager
	@echo "installing Addon Operator $(VERSION)..."
	@(source hack/determine-container-runtime.sh \
		&& export KUBECONFIG=$(KIND_KUBECONFIG) \
		&& $$KIND_COMMAND load image-archive \
			bin/image/addon-operator-manager.tar \
			--name addon-operator-e2e \
		&& kubectl apply -f config/deploy \
		&& yq eval '.spec.template.spec.containers[0].image = "$(IMAGE_ORG)/addon-operator-manager:$(VERSION)"' \
			config/deploy/deployment.yaml.tpl \
			| kubectl apply -f - \
		&& echo -e "\nwaiting for deployment/addon-operator..." \
		&& kubectl wait --for=condition=available deployment/addon-operator -n addon-operator --timeout=240s \
		&& echo) 2>&1 | sed 's/^/  /'
.PHONY: apply-ao

# ----------------
# Container Images
# ----------------

build-images: \
	build-image-addon-operator-manager
.PHONY: build-images

push-images: \
	push-image-addon-operator-manager
.PHONY: push-images

.SECONDEXPANSION:
build-image-%: bin/linux_amd64/$$*
	@echo "building image ${IMAGE_ORG}/$*:${VERSION}..."
	@(source hack/determine-container-runtime.sh \
		&& rm -rf "bin/image/$*" "bin/image/$*.tar" \
		&& mkdir -p "bin/image/$*" \
		&& cp -a "bin/linux_amd64/$*" "bin/image/$*" \
		&& cp -a "config/docker/$*.Dockerfile" "bin/image/$*/Dockerfile" \
		&& cp -a "config/docker/passwd" "bin/image/$*/passwd" \
		&& echo "building ${IMAGE_ORG}/$*:${VERSION}" \
		&& $$CONTAINER_COMMAND build -t "${IMAGE_ORG}/$*:${VERSION}" "bin/image/$*" \
		&& $$CONTAINER_COMMAND image save -o "bin/image/$*.tar" "${IMAGE_ORG}/$*:${VERSION}" \
		&& echo) 2>&1 | sed 's/^/  /'

push-image-%: build-image-$$*
	@echo "pushing image ${IMAGE_ORG}/$*:${VERSION}..."
	@(source hack/determine-container-runtime.sh \
		&& $$CONTAINER_COMMAND push "${IMAGE_ORG}/$*:${VERSION}" \
		&& echo pushed "${IMAGE_ORG}/$*:${VERSION}" \
		&& echo) 2>&1 | sed 's/^/  /'
