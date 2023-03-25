package controllers

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"

	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
)

type BackupSessionReconciler struct {
	Session
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
	r.Namespace = backupConf.Namespace

	// we don't want a copy because we will modify and update it.
	var target formolv1alpha1.Target
	var targetStatus *formolv1alpha1.TargetStatus
	var result error
	targetName := os.Getenv(formolv1alpha1.TARGET_NAME)
	if targetName == "" {
		panic("targetName is empty. That should not happen")
	}

	for i, t := range backupConf.Spec.Targets {
		if t.TargetName == targetName {
			target = t
			targetStatus = &(backupSession.Status.Targets[i])
			break
		}
	}

	// Do preliminary checks with the repository
	if err = r.SetResticEnv(backupConf); err != nil {
		r.Log.Error(err, "unable to set restic env")
		return ctrl.Result{}, err
	}

	var newSessionState formolv1alpha1.SessionState
	switch targetStatus.SessionState {
	case formolv1alpha1.New:
		// New session move to Initializing
		r.Log.V(0).Info("New session. Move to Initializing state")
		newSessionState = formolv1alpha1.Initializing
	case formolv1alpha1.Initializing:
		// Run the initializing Steps and then move to Initialized or Failure
		r.Log.V(0).Info("Start to run the backup initializing steps is any")
		// Runs the Steps functions in chroot env
		if err := r.runInitializeSteps(target); err != nil {
			r.Log.Error(err, "unable to run the initialization steps")
			newSessionState = formolv1alpha1.Failure
		} else {
			r.Log.V(0).Info("Done with the initializing Steps. Move to Initialized state")
			newSessionState = formolv1alpha1.Initialized
		}
	case formolv1alpha1.Running:
		// Actually do the backup and move to Waiting or Failure
		r.Log.V(0).Info("Running state. Do the backup")
		// Actually do the backup with restic
		newSessionState = formolv1alpha1.Waiting
		switch target.BackupType {
		case formolv1alpha1.JobKind:
			if backupResult, err := r.backupJob(backupSession.Name, target); err != nil {
				r.Log.Error(err, "unable to run backup job", "target", targetName)
				newSessionState = formolv1alpha1.Failure
			} else {
				r.Log.V(0).Info("Backup Job is over", "target", targetName, "snapshotID", backupResult.SnapshotId, "duration", backupResult.Duration)
				targetStatus.SnapshotId = backupResult.SnapshotId
				targetStatus.Duration = &metav1.Duration{Duration: time.Now().Sub(targetStatus.StartTime.Time)}
			}
		case formolv1alpha1.OnlineKind:
			backupPaths := strings.Split(os.Getenv(formolv1alpha1.BACKUP_PATHS), string(os.PathListSeparator))
			if backupResult, result := r.backupPaths(backupSession.Name, backupPaths); result != nil {
				r.Log.Error(result, "unable to backup paths", "target name", targetName, "paths", backupPaths)
				newSessionState = formolv1alpha1.Failure
			} else {
				r.Log.V(0).Info("Backup of the paths is over", "target name", targetName, "paths", backupPaths,
					"snapshotID", backupResult.SnapshotId, "duration", backupResult.Duration)
				targetStatus.SnapshotId = backupResult.SnapshotId
				targetStatus.Duration = &metav1.Duration{Duration: time.Now().Sub(targetStatus.StartTime.Time)}
			}
		}
		r.Log.V(0).Info("Backup is over and is a success. Move to Waiting state")
	case formolv1alpha1.Finalize:
		// Run the finalize Steps and move to Success or Failure
		r.Log.V(0).Info("Backup is over. Run the finalize steps is any")
		// Runs the finalize Steps functions in chroot env
		if result = r.runFinalizeSteps(target); result != nil {
			r.Log.Error(err, "unable to run finalize steps")
		}
		if targetStatus.SnapshotId == "" {
			newSessionState = formolv1alpha1.Failure
		} else {
			newSessionState = formolv1alpha1.Success
		}
	case formolv1alpha1.Success:
		// Target backup is a success
		r.Log.V(0).Info("Backup was a success")
	case formolv1alpha1.Failure:
		// Target backup is a failure
	}
	if newSessionState != "" {
		targetStatus.SessionState = newSessionState
		err := r.Status().Update(ctx, &backupSession)
		if err != nil {
			r.Log.Error(err, "unable to update BackupSession status")
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, result
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&formolv1alpha1.BackupSession{}).
		Complete(r)
}
