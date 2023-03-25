package controllers

import (
	"context"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type RestoreSessionReconciler struct {
	Session
	backupConf     formolv1alpha1.BackupConfiguration
	restoreSession formolv1alpha1.RestoreSession
}

func (r *RestoreSessionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log = log.FromContext(ctx)
	r.Context = ctx

	restoreSession := formolv1alpha1.RestoreSession{}
	err := r.Get(r.Context, req.NamespacedName, &restoreSession)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if len(restoreSession.Status.Targets) == 0 {
		r.Log.V(0).Info("RestoreSession still being initialized by the main controller. Wait for the next update...")
		return ctrl.Result{}, nil
	}
	r.restoreSession = restoreSession
	// We need the BackupConfiguration to get information about our restore target
	backupSession := formolv1alpha1.BackupSession{
		Spec:   restoreSession.Spec.BackupSessionRef.Spec,
		Status: restoreSession.Spec.BackupSessionRef.Status,
	}
	backupConf := formolv1alpha1.BackupConfiguration{}
	err = r.Get(r.Context, client.ObjectKey{
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
	r.backupConf = backupConf

	// we don't want a copy because we will modify and update it.
	var target formolv1alpha1.Target
	var restoreTargetStatus *formolv1alpha1.TargetStatus
	var backupTargetStatus formolv1alpha1.TargetStatus
	targetName := os.Getenv(formolv1alpha1.TARGET_NAME)

	for i, t := range backupConf.Spec.Targets {
		if t.TargetName == targetName {
			target = t
			restoreTargetStatus = &(restoreSession.Status.Targets[i])
			backupTargetStatus = backupSession.Status.Targets[i]
			break
		}
	}

	// Do preliminary checks with the repository
	if err = r.SetResticEnv(backupConf); err != nil {
		r.Log.Error(err, "unable to set restic env")
		return ctrl.Result{}, err
	}

	var newSessionState formolv1alpha1.SessionState
	switch restoreTargetStatus.SessionState {
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
		// Do the restore and move to Waiting once it is done.
		// The restore is different if the Backup was an OnlineKind or a JobKind
		switch target.BackupType {
		case formolv1alpha1.JobKind:
			r.Log.V(0).Info("restoring job backup", "target", target)
			if err := r.restoreJob(target, backupTargetStatus); err != nil {
				r.Log.Error(err, "unable to restore job", "target", target)
				newSessionState = formolv1alpha1.Failure
			} else {
				r.Log.V(0).Info("job backup restore was a success", "target", target)
				newSessionState = formolv1alpha1.Success
			}
		case formolv1alpha1.OnlineKind:
			// The initContainer will update the SessionState of the target
			// once it is done with the restore
			r.Log.V(0).Info("restoring online backup", "target", target)
			if err := r.restoreInitContainer(target); err != nil {
				r.Log.Error(err, "unable to create restore initContainer", "target", target)
				newSessionState = formolv1alpha1.Failure
			}
		}
	case formolv1alpha1.Finalize:
		r.Log.V(0).Info("We are done with the restore. Run the finalize steps")
		// Runs the finalize Steps functions in chroot env
		if err := r.runFinalizeSteps(target); err != nil {
			r.Log.Error(err, "unable to run finalize steps")
			newSessionState = formolv1alpha1.Failure
		} else {
			r.Log.V(0).Info("Ran the finalize steps. Restore was a success")
			newSessionState = formolv1alpha1.Success
		}
	}
	if newSessionState != "" {
		restoreTargetStatus.SessionState = newSessionState
		err := r.Status().Update(ctx, &restoreSession)
		if err != nil {
			r.Log.Error(err, "unable to update RestoreSession status")
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *RestoreSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&formolv1alpha1.RestoreSession{}).
		Complete(r)
}
