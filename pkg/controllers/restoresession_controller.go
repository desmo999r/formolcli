package controllers

import (
	"context"
	"fmt"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/desmo999r/formolcli/pkg/restic"
	formolcliutils "github.com/desmo999r/formolcli/pkg/utils"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
	"time"
)

type RestoreSessionReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	RestoreSession *formolv1alpha1.RestoreSession
	BackupSession  *formolv1alpha1.BackupSession
	BackupConf     *formolv1alpha1.BackupConfiguration
}

func (r *RestoreSessionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	time.Sleep(2 * time.Second)
	ctx := context.Background()
	log := r.Log.WithValues("restoresession", req.NamespacedName)
	r.RestoreSession = &formolv1alpha1.RestoreSession{}
	if err := r.Get(ctx, req.NamespacedName, r.RestoreSession); err != nil {
		log.Error(err, "unable to get restoresession")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.BackupSession = &formolv1alpha1.BackupSession{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: r.RestoreSession.Spec.BackupSessionRef.Namespace,
		Name:      r.RestoreSession.Spec.BackupSessionRef.Name}, r.BackupSession); err != nil {
		log.Error(err, "unable to get backupsession", "namespace", r.RestoreSession.Spec.BackupSessionRef.Namespace, "name", r.RestoreSession.Spec.BackupSessionRef.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.BackupConf = &formolv1alpha1.BackupConfiguration{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.BackupSession.Namespace,
		Name: r.BackupSession.Spec.Ref.Name}, r.BackupConf); err != nil {
		log.Error(err, "unable to get backupconfiguration")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	deploymentName := os.Getenv("POD_DEPLOYMENT")
	// we go reverse order compared to the backup session
	for i := len(r.BackupConf.Spec.Targets) - 1; i >= 0; i-- {
		target := r.BackupConf.Spec.Targets[i]
		switch target.Kind {
		case "Deployment":
			if target.Name == deploymentName {
				for i, status := range r.RestoreSession.Status.Targets {
					if status.Name == target.Name {
						log.V(0).Info("It's for us!", "target", target.Name)
						switch status.SessionState {
						case formolv1alpha1.New:
							log.V(0).Info("New session, run the beforeBackup hooks if any")
							result := formolv1alpha1.Running
							if err := formolcliutils.RunBeforeBackup(target); err != nil {
								result = formolv1alpha1.Failure
							}
							r.RestoreSession.Status.Targets[i].SessionState = result
							if err := r.Status().Update(ctx, r.RestoreSession); err != nil {
								log.Error(err, " unable to update restoresession status")
								return ctrl.Result{}, err
							}
						case formolv1alpha1.Running:
							log.V(0).Info("Running session. Do the restore")
							status.StartTime = &metav1.Time{Time: time.Now()}
							result := formolv1alpha1.Success

							repo := &formolv1alpha1.Repo{}
							if err := r.Get(ctx, client.ObjectKey{
								Namespace: r.BackupConf.Namespace,
								Name:      r.BackupConf.Spec.Repository.Name,
							}, repo); err != nil {
								log.Error(err, "unable to get Repo from BackupConf")
								return ctrl.Result{}, err
							}
							url := fmt.Sprintf("s3:http://%s/%s/%s-%s", repo.Spec.Backend.S3.Server, repo.Spec.Backend.S3.Bucket, strings.ToUpper(r.BackupConf.Namespace), strings.ToLower(r.BackupConf.Name))
							output, err := restic.RestorePaths(url, r.BackupSession.Status.Targets[i].SnapshotId)
							if err != nil {
								log.Error(err, "unable to restore deployment", "output", string(output))
								result = formolv1alpha1.Failure
							} else {
								duration := restic.GetRestoreResults(output)
								r.RestoreSession.Status.Targets[i].Duration = &metav1.Duration{Duration: duration}
							}
							r.RestoreSession.Status.Targets[i].SessionState = result
							log.V(1).Info("current restoresession status", "status", result)
							if err := r.Status().Update(ctx, r.RestoreSession); err != nil {
								log.Error(err, "unable to update restoresession status")
								return ctrl.Result{}, err
							}

						case formolv1alpha1.Failure, formolv1alpha1.Success:
							log.V(0).Info("Restore is over, run afterBackup hooks if any")
							formolcliutils.RunAfterBackup(target)
						}
					}
				}
			}
		}

	}
	return ctrl.Result{}, nil
}

func (r *RestoreSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&formolv1alpha1.RestoreSession{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool { return false },
			DeleteFunc: func(e event.DeleteEvent) bool { return false },
		}).
		Complete(r)
}
