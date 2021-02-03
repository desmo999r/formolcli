package controllers

import (
	"context"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type RestoreSessionReconciler struct {
	client.Client
	Log                 logr.Logger
	Scheme              *runtime.Scheme
	RestoreSession      *formolv1alpha1.RestoreSession
	BackupSession       *formolv1alpha1.BackupSession
	BackupConfiguration *formolv1alpha1.BackupConfiguration
}

func (r *RestoreSessionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("restoresession", req.NamespacedName)
	r.RestoreSession = &formolv1alpha1.RestoreSession{}
	if err := r.Get(ctx, req.NamespacedName, r.RestoreSession); err != nil {
		log.Error(err, "unable to get restoresession")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.BackupSession = &formolv1alpha1.BackupSession{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.RestoreSession.Spec.BackupSessionRef.Namespace,
		Name: r.RestoreSession.Spec.BackupSessionRef.Name}, r.BackupSession); err != nil {
		log.Error(err, "unable to get backupsession")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.BackupConfiguration = &formolv1alpha1.BackupConfiguration{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.BackupSession.Namespace,
		Name: r.BackupSession.Spec.Ref.Name}, r.BackupConfiguration); err != nil {
		log.Error(err, "unable to get backupconfiguration")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	deploymentName := os.Getenv("POD_DEPLOYMENT")
	for i := len(r.BackupConfiguration.Spec.Targets) - 1; i >= 0; i-- {
		target := r.BackupConfiguration.Spec.Targets[i]
		switch target.Kind {
		case "Deployment":
			if target.Name == deploymentName {
				log.V(0).Info("It's for us!", "target", target.Name)
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
