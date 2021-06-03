package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/desmo999r/formolcli/pkg/restic"
	formolcliutils "github.com/desmo999r/formolcli/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupSessionReconciler reconciles a BackupSession object
type BackupSessionReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

var _ reconcile.Reconciler = &BackupSessionReconciler{}

func (r *BackupSessionReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	time.Sleep(2 * time.Second)
	log := r.Log.WithValues("backupsession", req.NamespacedName)

	// your logic here
	backupSession := &formolv1alpha1.BackupSession{}
	if err := r.Get(ctx, req.NamespacedName, backupSession); err != nil {
		log.Error(err, "unable to get backupsession")
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	backupConf := &formolv1alpha1.BackupConfiguration{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: backupSession.Namespace,
		Name:      backupSession.Spec.Ref.Name,
	}, backupConf); err != nil {
		log.Error(err, "unable to get backupConfiguration")
		return reconcile.Result{}, client.IgnoreNotFound(err)
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
							return reconcile.Result{}, err
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
								return reconcile.Result{}, err
							}
							// TODO: check the command arguments for $(VAR_NAME) arguments. If some are found, try to expand them from
							// the Function.Spec EnvFrom and Env in that order
							pattern := regexp.MustCompile(`^\$\((\w+)\)$`)
							for i, arg := range function.Spec.Command[1:] {
								i++
								if match, _ := regexp.MatchString(`^\$\$`, arg); match {
									continue
								}
								if pattern.MatchString(arg) {
									arg = pattern.ReplaceAllString(arg, "$1")
									// TODO: Find arg in EnvFrom key and replace it by the value in Command[i]
									for _, envFrom := range function.Spec.EnvFrom {
										if envFrom.SecretRef != nil {
											secret := &corev1.Secret{}
											if err := r.Get(ctx, client.ObjectKey{
												Name:      envFrom.SecretRef.Name,
												Namespace: backupConf.Namespace,
											}, secret); err != nil {
												log.Error(err, "unable to get secret", "secret", envFrom.SecretRef.Name)
												return reconcile.Result{}, err
											}
											if val, ok := secret.Data[arg]; ok {
												log.V(1).Info("Found EnvFrom value for arg", "arg", arg)
												function.Spec.Command[i] = string(val)
											}

										}
										if envFrom.ConfigMapRef != nil {
											configMap := &corev1.ConfigMap{}
											if err := r.Get(ctx, client.ObjectKey{
												Name:      envFrom.ConfigMapRef.Name,
												Namespace: backupConf.Namespace,
											}, configMap); err != nil {
												log.Error(err, "unable to get configMap", "configMap", envFrom.ConfigMapRef.Name)
												return reconcile.Result{}, err
											}
											if val, ok := configMap.Data[arg]; ok {
												log.V(1).Info("Found EnvFrom value for arg", "arg", arg)
												function.Spec.Command[i] = val
											}
										}
									}
									for _, env := range function.Spec.Env {
										if env.Name == arg {
											if env.Value == "" {
												if env.ValueFrom != nil {
													if env.ValueFrom.SecretKeyRef != nil {
														secret := &corev1.Secret{}
														if err := r.Get(ctx, client.ObjectKey{
															Name:      env.ValueFrom.SecretKeyRef.Name,
															Namespace: backupConf.Namespace,
														}, secret); err != nil {
															log.Error(err, "unable to get secret", "secret", env.ValueFrom.SecretKeyRef.Name)
															return reconcile.Result{}, err
														}
														log.V(1).Info("Found Env value for arg", "arg", arg)
														function.Spec.Command[i] = string(secret.Data[env.ValueFrom.SecretKeyRef.Key])
													}
												}
											} else {
												function.Spec.Command[i] = env.Value
											}
										}
									}
								}
							}
							if err := formolcliutils.RunChroot(target.ContainerName != "", function.Spec.Command[0], function.Spec.Command[1:]...); err != nil {
								log.Error(err, "unable to run function command", "command", function.Spec.Command)
								result = formolv1alpha1.Failure
								break
							}
						}
						status.SessionState = result

						if err := r.Status().Update(ctx, backupSession); err != nil {
							log.Error(err, "unable to update backupsession status")
							return reconcile.Result{}, err
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
							return reconcile.Result{}, err
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
									return reconcile.Result{}, err
								}
								if err := formolcliutils.RunChroot(target.ContainerName != "", function.Spec.Command[0], function.Spec.Command[1:]...); err != nil {
									log.Error(err, "unable to run function command", "command", function.Spec.Command)
									result = formolv1alpha1.Failure
									break
								}
							}
						}
						status.SessionState = result

						if err := r.Status().Update(ctx, backupSession); err != nil {
							log.Error(err, "unable to update backupsession status")
							return reconcile.Result{}, err
						}

					case formolv1alpha1.Success, formolv1alpha1.Failure:
						log.V(0).Info("Backup is over")
					}
				}
			}
		}
	}
	return reconcile.Result{}, nil
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
