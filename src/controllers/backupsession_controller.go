package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"os"
	"path/filepath"

	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"github.com/desmo999r/formolcli/backup"
)

var (
	deploymentName = ""
)

func init() {
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		panic("No POD_NAMESPACE env var")
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config",))
		if err != nil {
			panic(err.Error())
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic("unable to get clientset")
	}

	hostname := os.Getenv("POD_NAME")
	if hostname == "" {
		panic("unable to get hostname")
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(hostname, metav1.GetOptions{})
	if err != nil {
		panic("unable to get pod")
	}

	podOwner := metav1.GetControllerOf(pod)
	replicasetList, err := clientset.AppsV1().ReplicaSets(namespace).List(metav1.ListOptions{
		FieldSelector: "metadata.name=" + string(podOwner.Name),
	})
	if err != nil {
		panic("unable to get replicaset" + err.Error())
	}
	for _, replicaset := range replicasetList.Items {
		replicasetOwner := metav1.GetControllerOf(&replicaset)
		deploymentName = replicasetOwner.Name
	}
}

// BackupSessionReconciler reconciles a BackupSession object
type BackupSessionReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *BackupSessionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("backupsession", req.NamespacedName)

	// your logic here
	backupSession := &formolv1alpha1.BackupSession{}
	if err := r.Get(ctx, req.NamespacedName, backupSession); err != nil {
		log.Error(err, "unable to get backupsession")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.V(1).Info("backupSession.Namespace", "namespace", backupSession.Namespace)
	log.V(1).Info("backupSession.Spec.Ref.Name", "name", backupSession.Spec.Ref.Name)
	backupConf := &formolv1alpha1.BackupConfiguration{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: backupSession.Namespace,
		Name:      backupSession.Spec.Ref.Name,
	}, backupConf); err != nil {
		log.Error(err, "unable to get backupConfiguration")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.V(1).Info("Found BackupConfiguration", "BackupConfiguration", backupConf)

	// Found the BackupConfiguration.
	switch backupConf.Spec.Target.Kind {
	case "Deployment":
		if err := backup.BackupDeployment("", backupConf.Spec.Paths); err != nil {
			log.Error(err, "unable to backup deployment")
			return ctrl.Result{}, nil
		}
	default:
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func (r *BackupSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&formolv1alpha1.BackupSession{}).
		Complete(r)
}

