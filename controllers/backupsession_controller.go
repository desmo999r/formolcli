package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"

	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
)

type BackupSessionReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	context.Context
}

func (r *BackupSessionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log = log.FromContext(ctx)
	r.Context = ctx

	backupSession := formolv1alpha1.BackupSession{}
	err := r.Get(ctx, req.NamespacedName, &backupSession)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if len(backupSession.Status.Targets) == 0 {
		// The main BackupSession controller hasn't assigned a backup task yet
		// Wait a bit
		r.Log.V(0).Info("No task has been assigned yet. Wait for the next update...")
		return ctrl.Result{}, nil
	}
	backupConf := formolv1alpha1.BackupConfiguration{}
	err = r.Get(ctx, client.ObjectKey{
		Namespace: backupSession.Spec.Ref.Namespace,
		Name:      backupSession.Spec.Ref.Name,
	}, &backupConf)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	targetName := os.Getenv(formolv1alpha1.TARGET_NAME)
	// we don't want a copy because we will modify and update it.
	currentTargetStatus := &(backupSession.Status.Targets[len(backupSession.Status.Targets)-1])
	currentTarget := backupConf.Spec.Targets[len(backupSession.Status.Targets)-1]
	var result error
	if currentTargetStatus.TargetName == targetName {
		// The current task is for us
		var newSessionState formolv1alpha1.SessionState
		switch currentTargetStatus.SessionState {
		case formolv1alpha1.New:
			r.Log.V(0).Info("New session, move to Initializing state")
			newSessionState = formolv1alpha1.Init
		case formolv1alpha1.Init:
			r.Log.V(0).Info("Start to run the backup initializing steps is any")
			// Runs the Steps functions in chroot env
			if result = r.runInitializeBackupSteps(currentTarget); result != nil {
				r.Log.Error(result, "unable to run the initialization steps")
				newSessionState = formolv1alpha1.Finalize
			} else {
				newSessionState = formolv1alpha1.Running
			}
		case formolv1alpha1.Running:
			r.Log.V(0).Info("Running state. Do the backup")
			// Actually do the backup with restic
			backupPaths := strings.Split(os.Getenv(formolv1alpha1.BACKUP_PATHS), string(os.PathListSeparator))
			if backupResult, result := r.backupPaths(backupSession.Name, backupPaths); result != nil {
				r.Log.Error(result, "unable to backup paths", "target name", targetName, "paths", backupPaths)
			} else {
				r.Log.V(0).Info("Backup of the paths is over", "target name", targetName, "paths", backupPaths,
					"snapshotID", backupResult.SnapshotId, "duration", backupResult.Duration)
				currentTargetStatus.SnapshotId = backupResult.SnapshotId
				currentTargetStatus.Duration = &metav1.Duration{Duration: time.Now().Sub(currentTargetStatus.StartTime.Time)}
			}
			newSessionState = formolv1alpha1.Finalize
		case formolv1alpha1.Finalize:
			r.Log.V(0).Info("Backup is over. Run the finalize steps is any")
			// Runs the finalize Steps functions in chroot env
			if result = r.runFinalizeBackupSteps(currentTarget); result != nil {
				r.Log.Error(err, "unable to run finalize steps")
			}
			if currentTargetStatus.SnapshotId == "" {
				newSessionState = formolv1alpha1.Failure
			} else {
				newSessionState = formolv1alpha1.Success
			}
		case formolv1alpha1.Success:
			r.Log.V(0).Info("Backup is over")
		case formolv1alpha1.Failure:
			r.Log.V(0).Info("Backup is over")
		}
		if newSessionState != "" {
			currentTargetStatus.SessionState = newSessionState
			err := r.Status().Update(ctx, &backupSession)
			if err != nil {
				r.Log.Error(err, "unable to update BackupSession status")
			}
			return ctrl.Result{}, err

		}
	}
	return ctrl.Result{}, result
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&formolv1alpha1.BackupSession{}).
		Complete(r)
}
