package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"

	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/desmo999r/formolcli/pkg/restic"
	formolcliutils "github.com/desmo999r/formolcli/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupSessionReconciler reconciles a BackupSession object
type BackupSessionReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *BackupSessionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	time.Sleep(2 * time.Second)
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

	deploymentName := os.Getenv("POD_DEPLOYMENT")
	for _, target := range backupConf.Spec.Targets {
		switch target.Kind {
		case "Deployment":
			if target.Name == deploymentName {
				for i, status := range backupSession.Status.Targets {
					if status.Name == target.Name {
						log.V(0).Info("It's for us!", "target", target)
						switch status.SessionState {
						case formolv1alpha1.New:
							// TODO: Run beforeBackup
							log.V(0).Info("New session, run the beforeBackup hooks if any")
							result := formolv1alpha1.Running
							if err := formolcliutils.RunBeforeBackup(target); err != nil {
								result = formolv1alpha1.Failure
							}
							backupSession.Status.Targets[i].SessionState = result
							log.V(1).Info("current backupSession status", "status", backupSession.Status)
							if err := r.Status().Update(ctx, backupSession); err != nil {
								log.Error(err, "unable to update backupsession status")
								return ctrl.Result{}, err
							}
						case formolv1alpha1.Running:
							log.V(0).Info("Running session. Do the backup")
							result := formolv1alpha1.Success
							status.StartTime = &metav1.Time{Time: time.Now()}
							output, err := restic.BackupPaths(backupSession.Name, target.Paths)
							if err != nil {
								log.Error(err, "unable to backup deployment", "output", string(output))
								result = formolv1alpha1.Failure
							} else {
								snapshotId := restic.GetBackupResults(output)
								backupSession.Status.Targets[i].SnapshotId = snapshotId
								backupSession.Status.Targets[i].Duration = &metav1.Duration{Duration: time.Now().Sub(backupSession.Status.Targets[i].StartTime.Time)}
							}
							backupSession.Status.Targets[i].SessionState = result
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
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool { return false },
			DeleteFunc: func(e event.DeleteEvent) bool { return false },
		}).
		Complete(r)
}
