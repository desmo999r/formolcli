package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
)

type BackupSessionReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *BackupSessionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log = log.FromContext(ctx)
	r.Log.V(0).Info("Enter Reconcile with req", "req", req)

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
		r.Log.V(0).Info("No task has been assigned yet. Wait a bit...")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	//	backupConf := formolv1alpha1.BackupConfiguration{}
	//	err := r.Get(ctx, client.ObjectKey {
	//		Namespace: backupSession.Spec.Ref.Namespace,
	//		Name: backupSession.Spec.Ref.Name,
	//	}, &backupConf)
	//	if err != nil {
	//		if errors.IsNotFound(err) {
	//			return ctrl.Result{}, nil
	//		}
	//		return ctrl.Result{}, err
	//	}

	targetName := os.Getenv(formolv1alpha1.TARGET_NAME)
	// we don't want a copy because we will modify and update it.
	currentTargetStatus := &(backupSession.Status.Targets[len(backupSession.Status.Targets)-1])
	if currentTargetStatus.TargetName == targetName {
		// The current task is for us
		switch currentTargetStatus.SessionState {
		case formolv1alpha1.New:
			r.Log.V(0).Info("New session, move to Initializing state")
			currentTargetStatus.SessionState = formolv1alpha1.Init
			err := r.Status().Update(ctx, &backupSession)
			if err != nil {
				r.Log.Error(err, "unable to update BackupSession status")
			}
			return ctrl.Result{}, err
		case formolv1alpha1.Init:
			r.Log.V(0).Info("Start to run the backup initializing steps is any")
			// Runs the Steps functions in chroot env
			result := formolv1alpha1.Running
			currentTargetStatus.SessionState = result
			err := r.Status().Update(ctx, &backupSession)
			if err != nil {
				r.Log.Error(err, "unable to update BackupSession status")
			}
			return ctrl.Result{}, err
		case formolv1alpha1.Running:
			r.Log.V(0).Info("Running state. Do the backup")
			// Actually do the backup with restic
			currentTargetStatus.SessionState = formolv1alpha1.Finalize
			err := r.Status().Update(ctx, &backupSession)
			if err != nil {
				r.Log.Error(err, "unable to update BackupSession status")
			}
			return ctrl.Result{}, err
		case formolv1alpha1.Finalize:
			r.Log.V(0).Info("Backup is over. Run the finalize steps is any")
			// Runs the finalize Steps functions in chroot env
			if currentTargetStatus.SnapshotId == "" {
				currentTargetStatus.SessionState = formolv1alpha1.Failure
			} else {
				currentTargetStatus.SessionState = formolv1alpha1.Success
			}
			err := r.Status().Update(ctx, &backupSession)
			if err != nil {
				r.Log.Error(err, "unable to update BackupSession status")
			}
			return ctrl.Result{}, err
		case formolv1alpha1.Success:
		case formolv1alpha1.Failure:
			r.Log.V(0).Info("Backup is over")
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&formolv1alpha1.BackupSession{}).
		Complete(r)
}
