SHELL=/bin/bash
.SHELLFLAGS=-euo pipefail -c

SHORT_SHA=$(shell git rev-parse --short HEAD)
VERSION?=${SHORT_SHA}
IMAGE_ORG="quay.io/app-sre"

# App Interface specific push-images target, to run within a docker container.
app-interface-push-images:
	@echo "-------------------------------------------------"
	@echo "running in app-interface-push-images container..."
	@echo "-------------------------------------------------"
	$(eval IMAGE_NAME := app-interface-push-images)
	@(docker build -t "${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}" -f "config/images/${IMAGE_NAME}.Containerfile" --pull .; \
		docker run --rm \
			--privileged \
			-e JENKINS_HOME=${JENKINS_HOME} \
			-e QUAY_USER=${QUAY_USER} \
			-e QUAY_TOKEN=${QUAY_TOKEN} \
			-e VERSION=${VERSION} \
			-e IMAGE_ORG="${IMAGE_ORG}" \
			-e REMOTE_PHASE_MANAGER_IMAGE="${IMAGE_ORG}/package-operator-hs-connector:${VERSION}" \
			-e REMOTE_PHASE_PACKAGE_IMAGE="${IMAGE_ORG}/package-operator-hs-package:${VERSION}" \
			-e CLI_IMAGE="${IMAGE_ORG}/package-operator-cli:${VERSION}" \
			-e PKO_PACKAGE_NAMESPACE_OVERRIDE="openshift-package-operator" \
			"${IMAGE_ORG}/${IMAGE_NAME}:${VERSION}" \
			./mage build:pushImages; \
	echo) 2>&1 | sed 's/^/  /'
.PHONY: app-interface-push-images
