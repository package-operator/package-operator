package bootstrap

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/cmd/package-operator-manager/bootstrap/fix"
)

type runChecker interface {
	Check(ctx context.Context, fc fix.Context) (bool, error)
	Run(ctx context.Context, fc fix.Context) error
}

// Fixes PKO problems on the cluster by checking conditions and applying appropriate fixes as needed.
type fixer struct {
	log          logr.Logger
	client       client.Client
	pkoNamespace string
	fixes        []runChecker
}

func newFixer(c client.Client, log logr.Logger, pkoNamespace string) *fixer {
	return &fixer{
		log:          log.WithName("fixer"),
		client:       c,
		pkoNamespace: pkoNamespace,

		// Order matters here, the fixes are checked against and applied sequentially one-by-one.
		fixes: []runChecker{
			&fix.CRDPluralizationFix{},
			&fix.ControllerOfVersion{},
			&fix.RevisionDroppedFix{},
		},
	}
}

// fix iterates through all registered fixes/runCheckers, runs their checks to determine if
// the fix has to be executed and if so, it executes the fix.
func (f *fixer) fix(ctx context.Context) error {
	for _, fixRunChecker := range f.fixes {
		fixName := reflect.TypeOf(fixRunChecker).String()
		log := f.log.WithValues("fix", fixName)

		fc := fix.Context{
			Log:          log,
			Client:       f.client,
			PKONamespace: f.pkoNamespace,
		}

		// run eligibility check to find out if the fix should be executed
		applicable, err := fixRunChecker.Check(ctx, fc)
		if err != nil {
			return fmt.Errorf("check failed for fix %s: %w", fixName, err)
		}
		log.Info("checked", "applicable", applicable)

		// skip fix, if `.Check(...)` said that the fix is not applicable
		if !applicable {
			continue
		}

		// execute fix
		if err := fixRunChecker.Run(ctx, fc); err != nil {
			return fmt.Errorf("fix failed for fix %s: %w", fixName, err)
		}
		log.Info("executed")
	}

	return nil
}
