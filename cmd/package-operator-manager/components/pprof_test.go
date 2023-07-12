package components

import "testing"

func TestSetupPPROF(t *testing.T) {
	t.Parallel()
	_ = newPPROFServer(":9999")
}
