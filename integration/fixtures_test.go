package integration_test

import "time"

var (
	referenceAddonCatalogSourceImageWorking = "quay.io/osd-addons/reference-addon-index@sha256:58cb1c4478a150dc44e6c179d709726516d84db46e4e130a5227d8b76456b5bd"
	referenceAddonCatalogSourceImageBroken  = "quay.io/osd-addons/reference-addon-index@sha256:9e6306e310d585610d564412780d58ec54cb24a67d7cdabfc067ab733295010a"

	defaultAddonDeletionTimeout     = 2 * time.Minute
	defaultAddonAvailabilityTimeout = 5 * time.Minute
)
