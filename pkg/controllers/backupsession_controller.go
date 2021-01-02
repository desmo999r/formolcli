package controllers

import (
	"time"
	"sort"
	"encoding/json"
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"os"
	"path/filepath"

	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"github.com/desmo999r/formolcli/pkg/backup"
	formolutils "github.com/desmo999r/formol/pkg/utils"
)

var (
	deploymentName = ""
	sessionState = ".metadata.state"
)

func init() {
	log := zap.New(zap.UseDevMode(true)).WithName("init")

	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		return
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config",))
		if err != nil {
			log.Error(err, "unable to get config")
			panic(err.Error())
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic("unable to get clientset")
	}

	hostname := os.Getenv("POD_NAME")
	if hostname == "" {
		panic("unable to get hostname")
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), hostname, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "unable to get pod")
		panic("unable to get pod")
	}

	podOwner := metav1.GetControllerOf(pod)
	replicasetList, err := clientset.AppsV1().ReplicaSets(namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: "metadata.name=" + string(podOwner.Name),
	})
	if err != nil {
		panic("unable to get replicaset" + err.Error())
	}
	for _, replicaset := range replicasetList.Items {
		replicasetOwner := metav1.GetControllerOf(&replicaset)
		deploymentName = replicasetOwner.Name
	}
}

// BackupSessionReconciler reconciles a BackupSession object
type BackupSessionReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *BackupSessionReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("backupsession", req.NamespacedName)

	// your logic here
	backupSession := &formolv1alpha1.BackupSession{}
	if err := r.Get(ctx, req.NamespacedName, backupSession); err != nil {
		log.Error(err, "unable to get backupsession")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	finalizerName := "finalizer.backupsession.formol.desmojim.fr"

	if backupSession.ObjectMeta.DeletionTimestamp.IsZero() {
		if !formolutils.ContainsString(backupSession.ObjectMeta.Finalizers, finalizerName) {
			backupSession.ObjectMeta.Finalizers = append(backupSession.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(context.Background(), backupSession); err != nil {
				log.Error(err, "unable to append finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		log.V(0).Info("backupsession being deleted", "backupsession", backupSession.Name)
		if formolutils.ContainsString(backupSession.ObjectMeta.Finalizers, finalizerName) {
			if err := r.deleteExternalResources(backupSession); err != nil {
				return ctrl.Result{}, err
			}
		}
		backupSession.ObjectMeta.Finalizers = formolutils.RemoveString(backupSession.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(context.Background(), backupSession); err != nil {
			log.Error(err, "unable to remove finalizer")
			return ctrl.Result{}, err
		}
		// We have been deleted. Return here
		return ctrl.Result{}, nil
	}

	log.V(1).Info("backupSession.Namespace", "namespace", backupSession.Namespace)
	log.V(1).Info("backupSession.Spec.Ref.Name", "name", backupSession.Spec.Ref.Name)
	if backupSession.Status.BackupSessionState != "" {
		log.V(0).Info("State is not null. Skipping", "state", backupSession.Status.BackupSessionState)
		return ctrl.Result{}, nil
	}
	backupConf := &formolv1alpha1.BackupConfiguration{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: backupSession.Namespace,
		Name:      backupSession.Spec.Ref.Name,
	}, backupConf); err != nil {
		log.Error(err, "unable to get backupConfiguration")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Found the BackupConfiguration.
	log.V(1).Info("Found BackupConfiguration", "BackupConfiguration", backupConf.Name)

	backupDeployment := func(target formolv1alpha1.Target) error {
		log.V(0).Info("before", "backupsession", backupSession)
		backupSession.Status.BackupSessionState = formolv1alpha1.Running
		if err := r.Client.Status().Update(ctx, backupSession); err != nil {
			log.Error(err, "unable to update status", "backupsession", backupSession)
			return err
		}
		// Preparing for backup
		c := make(chan []byte)

		go func(){
			for msg := range c {
				var dat map[string]interface{}
				if err := json.Unmarshal(msg, &dat); err != nil {
					log.Error(err, "unable to unmarshal json", "msg", msg)
					continue
				}
				log.V(1).Info("message on stdout", "stdout", dat)
				if message_type, ok := dat["message_type"]; ok && message_type == "summary"{
					backupSession.Status.SnapshotId = dat["snapshot_id"].(string)
					backupSession.Status.Duration = &metav1.Duration{Duration : time.Duration(dat["total_duration"].(float64) * 1000) * time.Millisecond}
				}
			}
		}()
		result := formolv1alpha1.Failure
		defer func() {
			close(c)
			backupSession.Status.BackupSessionState = result
			if err := r.Status().Update(ctx, backupSession); err != nil {
				log.Error(err, "unable to update status")
			}
		}()
		// do the backup
		backupSession.Status.StartTime = &metav1.Time {Time: time.Now()}
		if err := backup.BackupDeployment("", target.Paths, c); err != nil {
			log.Error(err, "unable to backup deployment")
			return err
		}
		result = formolv1alpha1.Success

		// cleanup old backups
		backupSessionList := &formolv1alpha1.BackupSessionList{}
		if err := r.List(ctx, backupSessionList, client.InNamespace(backupConf.Namespace), client.MatchingFieldsSelector{fields.SelectorFromSet(fields.Set{sessionState: "Success"})}); err != nil {
			return nil
		}
		if len(backupSessionList.Items) < 2 {
			// Not enough backupSession to proceed
			return nil
		}

		sort.Slice(backupSessionList.Items, func(i, j int) bool {
			return backupSessionList.Items[i].Status.StartTime.Time.Unix() > backupSessionList.Items[j].Status.StartTime.Time.Unix()
		})

		type KeepBackup struct {
			Counter int32
			Last time.Time
		}

		var lastBackups, dailyBackups, weeklyBackups, monthlyBackups, yearlyBackups KeepBackup
		lastBackups.Counter = backupConf.Spec.Keep.Last
		dailyBackups.Counter = backupConf.Spec.Keep.Daily
		weeklyBackups.Counter = backupConf.Spec.Keep.Weekly
		monthlyBackups.Counter = backupConf.Spec.Keep.Monthly
		yearlyBackups.Counter = backupConf.Spec.Keep.Yearly
		for _, session := range backupSessionList.Items {
			if session.Spec.Ref.Name != backupConf.Name {
				continue
			}
			deleteSession := true
			if lastBackups.Counter > 0 {
				log.V(1).Info("Keep backup", "last", session.Status.StartTime)
				lastBackups.Counter--
				deleteSession = false
			}
			if dailyBackups.Counter > 0 {
				if session.Status.StartTime.Time.YearDay() != dailyBackups.Last.YearDay() {
					log.V(1).Info("Keep backup", "daily", session.Status.StartTime)
					dailyBackups.Counter--
					dailyBackups.Last = session.Status.StartTime.Time
					deleteSession = false
				}
			}
			if weeklyBackups.Counter > 0 {
				if session.Status.StartTime.Time.Weekday().String() == "Sunday" && session.Status.StartTime.Time.YearDay() != weeklyBackups.Last.YearDay() {
					log.V(1).Info("Keep backup", "weekly", session.Status.StartTime)
					weeklyBackups.Counter--
					weeklyBackups.Last = session.Status.StartTime.Time
					deleteSession = false
				}
			}
			if monthlyBackups.Counter > 0 {
				if session.Status.StartTime.Time.Day() == 1 && session.Status.StartTime.Time.Month() != monthlyBackups.Last.Month() {
					log.V(1).Info("Keep backup", "monthly", session.Status.StartTime)
					monthlyBackups.Counter--
					monthlyBackups.Last = session.Status.StartTime.Time
					deleteSession = false
				}
			}
			if yearlyBackups.Counter > 0 {
				if session.Status.StartTime.Time.YearDay() == 1 && session.Status.StartTime.Time.Year() != yearlyBackups.Last.Year() {
					log.V(1).Info("Keep backup", "yearly", session.Status.StartTime)
					yearlyBackups.Counter--
					yearlyBackups.Last = session.Status.StartTime.Time
					deleteSession = false
				}
			}
			if deleteSession {
				log.V(1).Info("Delete session", "delete", session.Status.StartTime)
				if err := r.Delete(ctx, &session); err != nil {
					log.Error(err, "unable to delete backupsession", "session", session.Name)
					// we don't return anything, we keep going
				}
			}
		}
		return nil
	}

	for _, target := range backupConf.Spec.Targets {
		switch target.Kind {
		case "Deployment":
			if target.Name == deploymentName {
				log.V(0).Info("It's for us!", "target", target)
				return ctrl.Result{}, backupDeployment(target)
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *BackupSessionReconciler) deleteExternalResources(backupSession *formolv1alpha1.BackupSession) error {
	if err := backup.DeleteSnapshot("", backupSession.Status.SnapshotId); err != nil {
		return err
	}
	return nil
}

func (r *BackupSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &formolv1alpha1.BackupSession{}, sessionState, func(rawObj runtime.Object) []string {
		session := rawObj.(*formolv1alpha1.BackupSession)
		return []string{string(session.Status.BackupSessionState)}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&formolv1alpha1.BackupSession{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}). // Don't reconcile when status gets updated
		Complete(r)
}

