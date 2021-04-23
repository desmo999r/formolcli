package controllers

import (
	"context"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	formolcliutils "github.com/desmo999r/formolcli/pkg/utils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
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
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *RestoreSessionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("restoresession", req.NamespacedName)

	restoreSession := &formolv1alpha1.RestoreSession{}
	if err := r.Get(ctx, req.NamespacedName, restoreSession); err != nil {
		log.Error(err, "unable to get restoresession")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	backupSession := &formolv1alpha1.BackupSession{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: restoreSession.Namespace,
		Name:      restoreSession.Spec.Ref,
	}, backupSession); err != nil {
		log.Error(err, "unable to get backupsession")
		return ctrl.Result{}, err
	}
	backupConf := &formolv1alpha1.BackupConfiguration{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: backupSession.Namespace,
		Name:      backupSession.Spec.Ref,
	}, backupConf); err != nil {
		log.Error(err, "unable to get backupConfiguration")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	deploymentName := os.Getenv(formolv1alpha1.TARGET_NAME)
	currentTargetStatus := &(restoreSession.Status.Targets[len(restoreSession.Status.Targets)-1])
	currentTarget := backupConf.Spec.Targets[len(restoreSession.Status.Targets)-1]
	switch currentTarget.Kind {
	case formolv1alpha1.SidecarKind:
		if currentTarget.Name == deploymentName {
			switch currentTargetStatus.SessionState {
			case formolv1alpha1.Running:
				log.V(0).Info("It's for us!")
				podName := os.Getenv(formolv1alpha1.POD_NAME)
				podNamespace := os.Getenv(formolv1alpha1.POD_NAMESPACE)
				pod := &corev1.Pod{}
				if err := r.Get(ctx, client.ObjectKey{
					Namespace: podNamespace,
					Name:      podName,
				}, pod); err != nil {
					log.Error(err, "unable to get pod", "name", podName, "namespace", podNamespace)
					return ctrl.Result{}, err
				}
				for _, containerStatus := range pod.Status.ContainerStatuses {
					if !containerStatus.Ready {
						log.V(0).Info("Not all the containers in the pod are ready. Reschedule", "name", containerStatus.Name)
						return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
					}
				}
				log.V(0).Info("All the containers in the pod are ready. Time to run the restore steps (in reverse order)")
				// We iterate through the steps in reverse order
				result := formolv1alpha1.Success
				for i := range currentTarget.Steps {
					step := currentTarget.Steps[len(currentTarget.Steps)-1-i]
					backupFunction := &formolv1alpha1.Function{}
					if err := r.Get(ctx, client.ObjectKey{
						Namespace: backupConf.Namespace,
						Name:      step.Name,
					}, backupFunction); err != nil {
						log.Error(err, "unable to get backup function")
						return ctrl.Result{}, err
					}
					// We got the backup function corresponding to the step from the BackupConfiguration
					// Now let's try to get the restore function is there is one
					restoreFunction := &formolv1alpha1.Function{}
					if restoreFunctionName, exists := backupFunction.Annotations[formolv1alpha1.RESTORE_ANNOTATION]; exists {
						log.V(0).Info("got restore function", "name", restoreFunctionName)
						if err := r.Get(ctx, client.ObjectKey{
							Namespace: backupConf.Namespace,
							Name:      restoreFunctionName,
						}, restoreFunction); err != nil {
							log.Error(err, "unable to get restore function")
							continue
						}
					} else {
						if strings.HasPrefix(backupFunction.Name, "backup-") {
							log.V(0).Info("backupFunction starts with 'backup-'", "name", backupFunction.Name)
							if err := r.Get(ctx, client.ObjectKey{
								Namespace: backupConf.Namespace,
								Name:      strings.Replace(backupFunction.Name, "backup-", "restore-", 1),
							}, restoreFunction); err != nil {
								log.Error(err, "unable to get restore function")
								continue
							}
						}
					}
					if len(restoreFunction.Spec.Command) > 1 {
						log.V(0).Info("Running the restore function", "name", restoreFunction.Name, "command", restoreFunction.Spec.Command)
						if err := formolcliutils.RunChroot(restoreFunction.Spec.Command[0], restoreFunction.Spec.Command[1:]...); err != nil {
							log.Error(err, "unable to run function command", "command", restoreFunction.Spec.Command)
							result = formolv1alpha1.Failure
							break
						} else {
							log.V(0).Info("Restore command is successful")
						}
					}
				}
				// We are done with the restore of this target. We flag it as success of failure
				// so that we can move to the next step
				currentTargetStatus.SessionState = result
				if err := r.Status().Update(ctx, restoreSession); err != nil {
					log.Error(err, "unable to update restoresession")
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
