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

	deploymentName := os.Getenv(formolv1alpha1.TARGET_NAME)
	for _, target := range backupConf.Spec.Targets {
		switch target.Kind {
		case formolv1alpha1.SidecarKind:
			if target.Name == deploymentName {
				// We are involved in that Backup, let's see if it's our turn
				status := &(backupSession.Status.Targets[len(backupSession.Status.Targets)-1])
				if status.Name == deploymentName {
					log.V(0).Info("It's for us!", "target", target)
					switch status.SessionState {
					case formolv1alpha1.New:
						log.V(0).Info("New session, move to Initializing state")
						status.SessionState = formolv1alpha1.Init
						if err := r.Status().Update(ctx, backupSession); err != nil {
							log.Error(err, "unable to update backupsession status")
							return ctrl.Result{}, err
						}
					case formolv1alpha1.Init:
						log.V(0).Info("Start to run the backup initializing steps if any")
						result := formolv1alpha1.Running
						for _, step := range target.Steps {
							if step.Finalize != nil && *step.Finalize == true {
								continue
							}
							function := &formolv1alpha1.Function{}
							if err := r.Get(ctx, client.ObjectKey{
								Name:      step.Name,
								Namespace: backupConf.Namespace,
							}, function); err != nil {
								log.Error(err, "unable to get function", "function", step.Name)
								return ctrl.Result{}, err
							}
							if err := formolcliutils.RunChroot(function.Spec.Command[0], function.Spec.Command[1:]...); err != nil {
								log.Error(err, "unable to run function command", "command", function.Spec.Command)
								result = formolv1alpha1.Failure
								break
							}
						}
						status.SessionState = result

						if err := r.Status().Update(ctx, backupSession); err != nil {
							log.Error(err, "unable to update backupsession status")
							return ctrl.Result{}, err
						}
					case formolv1alpha1.Running:
						log.V(0).Info("Running session. Do the backup")
						result := formolv1alpha1.Finalize
						status.StartTime = &metav1.Time{Time: time.Now()}
						output, err := restic.BackupPaths(backupSession.Name, target.Paths)
						if err != nil {
							log.Error(err, "unable to backup deployment", "output", string(output))
							result = formolv1alpha1.Failure
						} else {
							snapshotId := restic.GetBackupResults(output)
							status.SnapshotId = snapshotId
							status.Duration = &metav1.Duration{Duration: time.Now().Sub(status.StartTime.Time)}
						}
						status.SessionState = result
						log.V(1).Info("current backupSession status", "status", backupSession.Status)
						if err := r.Status().Update(ctx, backupSession); err != nil {
							log.Error(err, "unable to update backupsession status")
							return ctrl.Result{}, err
						}
					case formolv1alpha1.Finalize:
						log.V(0).Info("Start to run the backup finalizing steps if any")
						result := formolv1alpha1.Success
						for _, step := range target.Steps {
							if step.Finalize != nil && *step.Finalize == true {
								function := &formolv1alpha1.Function{}
								if err := r.Get(ctx, client.ObjectKey{
									Name:      step.Name,
									Namespace: backupConf.Namespace,
								}, function); err != nil {
									log.Error(err, "unable to get function", "function", step.Name)
									return ctrl.Result{}, err
								}
								if err := formolcliutils.RunChroot(function.Spec.Command[0], function.Spec.Command[1:]...); err != nil {
									log.Error(err, "unable to run function command", "command", function.Spec.Command)
									result = formolv1alpha1.Failure
									break
								}
							}
						}
						status.SessionState = result

						if err := r.Status().Update(ctx, backupSession); err != nil {
							log.Error(err, "unable to update backupsession status")
							return ctrl.Result{}, err
						}

					case formolv1alpha1.Success, formolv1alpha1.Failure:
						log.V(0).Info("Backup is over")
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
