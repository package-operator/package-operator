package components

import "testing"

func TestSetupPPROF(_ *testing.T) {
	_ = newPPROFServer(":9999")
}
