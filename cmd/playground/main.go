package main

import (
	"context"
	"fmt"
	"reflect"

	"go.uber.org/zap/zapcore"
	"pkg.package-operator.run/boxcutter/machinery"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	apis "package-operator.run/apis"
	"package-operator.run/internal/constants"
)

func main() {
	zapOpts := zap.Options{
		Development: false,
		Level:       zapcore.DebugLevel,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	ourScheme := runtime.NewScheme()
	schemeBuilder := runtime.SchemeBuilder{
		scheme.AddToScheme,
		apis.AddToScheme,
	}
	if err := schemeBuilder.AddToScheme(ourScheme); err != nil {
		panic(err)
	}

	config := ctrl.GetConfigOrDie()
	c, err := client.New(config, client.Options{
		Scheme: ourScheme,
	})
	must(err)

	ctx := context.Background()

	dc, err := discovery.NewDiscoveryClientForConfig(config)
	must(err)

	objectEngine := machinery.NewObjectEngine(
		ourScheme,
		c,
		c,
		machinery.NewComparator(
			dc,
			ourScheme,
			constants.FieldOwner,
		),
		constants.FieldOwner,
		constants.SystemPrefix,
	)

	owner := &corev1.ConfigMap{}
	must(c.Get(ctx, client.ObjectKey{
		Namespace: "default",
		Name:      "kube-root-ca.crt",
	}, owner))

	res, err := objectEngine.Reconcile(ctx, 1, &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"namespace": "default",
				"name":      "child",
			},
		},
	})
	must(err)

	fmt.Println(reflect.TypeOf(res.Object()))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
