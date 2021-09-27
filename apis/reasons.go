package apis

const (
	// AddonOperator condition reasons

	// Addon operator is ready
	AddonOperatorReasonReady = "AddonOperatorReady"

	// Addon operator has paused reconciliation
	AddonOperatorReasonPaused = "AddonOperatorPaused"

	// Addon operator has resumed reconciliation
	AddonOperatorReasonUnpaused = "AddonOperatorUnpaused"

	// Addon condition reasons

	// Addon as fully reconciled
	AddonReasonFullyReconciled = "FullyReconciled"

	// Addon is terminating
	AddonReasonTerminating = "Terminating"

	// Addon has a configurtion error
	AddonReasonConfigError = "ConfigurationError"

	// Addon has paused reconciliation
	AddonReasonPaused = "AddonPaused"

	// Addon has an unready Catalog source
	AddonReasonUnreadyCatalogSource = "UnreadyCatalogSource"

	// Addon has colliding namespaces
	AddonReasonCollidedNamespaces = "CollidedNamespaces"

	// Addon has unready namespaces
	AddonReasonUnreadyNamespaces = "UnreadyNamespaces"

	// Addon has unready CSV
	AddonReasonUnreadyCSV = "UnreadyCSV"
)
