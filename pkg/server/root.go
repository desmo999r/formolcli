package server

import (
	//	"flag"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/desmo999r/formolcli/pkg/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("server")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = formolv1alpha1.AddToScheme(scheme)
}

func Server() {
	//	var metricsAddr string
	//	var enableLeaderElection bool
	//	flag.StringVar(&metricsAddr, "metrics-addr", ":8082", "The address the metric endpoint binds to.")
	//	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
	//		"Enable leader election for controller manager. "+
	//			"Enabling this will ensure there is only one active controller manager.")
	//	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	config, err := ctrl.GetConfig()
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: scheme,
		//		MetricsBindAddress: metricsAddr,
		//		Port:               9443,
		//		LeaderElection:     enableLeaderElection,
		//		LeaderElectionID:   "12345.desmojim.fr",
		Namespace: os.Getenv("POD_NAMESPACE"),
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	// BackupSession controller
	if err = (&controllers.BackupSessionReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("BackupSession"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BackupSession")
		os.Exit(1)
	}

	// RestoreSession controller
	if err = (&controllers.RestoreSessionReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("RestoreSession"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RestoreSession")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running the manager")
		os.Exit(1)
	}
}
