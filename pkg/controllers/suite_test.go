package controllers

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	operatorv1 "sigs.k8s.io/cluster-api/exp/operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
)

func TestMain(m *testing.M) {
	t := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "manifests")},
	}

	utilruntime.Must(configv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(operatorv1.AddToScheme(scheme.Scheme))

	var err error
	if cfg, err = t.Start(); err != nil {
		log.Fatal(err)
	}

	mgr, err := manager.New(cfg, manager.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		log.Fatal(err)
	}

	mgrCtx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := mgr.Start(mgrCtx); err != nil {
			log.Fatal(err)
		}
	}()
	defer cancel()

	k8sClient = mgr.GetClient()

	code := m.Run()

	if err := t.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	os.Exit(code)
}
