package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"

	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/desmo999r/formolcli/pkg/backup"
	formolcliutils "github.com/desmo999r/formolcli/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	deploymentName = ""
)

func init() {
	log := zap.New(zap.UseDevMode(true)).WithName("init")

	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		return
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			log.Error(err, "unable to get config")
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

	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), hostname, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "unable to get pod")
		panic("unable to get pod")
	}

	podOwner := metav1.GetControllerOf(pod)
	replicasetList, err := clientset.AppsV1().ReplicaSets(namespace).List(context.Background(), metav1.ListOptions{
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
	backupConf := &formolv1alpha1.BackupConfiguration{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: backupSession.Namespace,
		Name:      backupSession.Spec.Ref.Name,
	}, backupConf); err != nil {
		log.Error(err, "unable to get backupConfiguration")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	for _, target := range backupConf.Spec.Targets {
		switch target.Kind {
		case "Deployment":
			if target.Name == deploymentName {
				for i, status := range backupSession.Status.Targets {
					if status.Name == target.Name {
						log.V(0).Info("It's for us!", "target", target)
						switch status.BackupState {
						case formolv1alpha1.New:
							// TODO: Run beforeBackup
							log.V(0).Info("New session, run the beforeBackup hooks if any")
							result := formolv1alpha1.Running
							if err := formolcliutils.RunBeforeBackup(target); err != nil {
								result = formolv1alpha1.Failure
							}
							backupSession.Status.Targets[i].BackupState = result
							log.V(1).Info("current backupSession status", "status", backupSession.Status)
							if err := r.Status().Update(ctx, backupSession); err != nil {
								log.Error(err, "unable to update backupsession status")
								return ctrl.Result{}, err
							}
						case formolv1alpha1.Running:
							log.V(0).Info("Running session. Do the backup")
							result := formolv1alpha1.Success
							status.StartTime = &metav1.Time{Time: time.Now()}
							output, err := backup.BackupPaths(backupSession.Name, target.Paths)
							if err != nil {
								log.Error(err, "unable to backup deployment", "output", string(output))
								result = formolv1alpha1.Failure
							} else {
								snapshotId, duration := backup.GetBackupResults(output)
								backupSession.Status.Targets[i].SnapshotId = snapshotId
								backupSession.Status.Targets[i].Duration = &metav1.Duration{Duration: duration}
							}
							backupSession.Status.Targets[i].BackupState = result
							log.V(1).Info("current backupSession status", "status", backupSession.Status)
							if err := r.Status().Update(ctx, backupSession); err != nil {
								log.Error(err, "unable to update backupsession status")
								return ctrl.Result{}, err
							}
						case formolv1alpha1.Success, formolv1alpha1.Failure:
							// I decided not to flag the backup as a failure if the AfterBackup command fail. But maybe I'm wrong
							log.V(0).Info("Backup is over, run the afterBackup hooks if any")
							formolcliutils.RunAfterBackup(target)
						}
					}
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *BackupSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&formolv1alpha1.BackupSession{}).
		//WithEventFilter(predicate.GenerationChangedPredicate{}). // Don't reconcile when status gets updated
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool { return false },
			DeleteFunc: func(e event.DeleteEvent) bool { return false },
		}). // Don't reconcile when status gets updated
		Complete(r)
}
