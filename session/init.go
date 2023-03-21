package session

import (
	"context"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	config *rest.Config
	scheme *runtime.Scheme
	cl     client.Client
	logger logr.Logger
	ctx    context.Context
)

func init() {
	logger = zap.New(zap.UseDevMode(true))
	ctx = context.Background()
	log := logger.WithName("InitBackupSession")
	ctrl.SetLogger(logger)
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			log.Error(err, "unable to get config")
			os.Exit(1)
		}
	}
	scheme = runtime.NewScheme()
	_ = formolv1alpha1.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)
	cl, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		log.Error(err, "unable to get client")
		os.Exit(1)
	}
}
