package composition

import (
	"go.uber.org/fx"
	"testing"
)

func TestFxDependencyGraph(t *testing.T) {
	if err := fx.ValidateApp(Module); err != nil {
		t.Fatal(err)
	}
}
