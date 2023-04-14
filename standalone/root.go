package standalone

import (
	"context"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/desmo999r/formolcli/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"os/exec"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strconv"
	"strings"
	"time"
)

const (
	BACKUPSESSION_PREFIX = "bs"
)

var (
	session controllers.Session
)

func init() {
	session.Log = zap.New(zap.UseDevMode(true))
	session.Context = context.Background()
	log := session.Log.WithName("InitBackupSession")
	ctrl.SetLogger(session.Log)
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			log.Error(err, "unable to get config")
			os.Exit(1)
		}
	}
	session.Scheme = runtime.NewScheme()
	_ = formolv1alpha1.AddToScheme(session.Scheme)
	_ = clientgoscheme.AddToScheme(session.Scheme)
	session.Client, err = client.New(config, client.Options{Scheme: session.Scheme})
	if err != nil {
		log.Error(err, "unable to get client")
		os.Exit(1)
	}
}

func StartRestore(
	restoreSessionName string,
	restoreSessionNamespace string,
	targetName string) {
	log := session.Log.WithName("StartRestore")
	if err := session.CheckRepo(); err != nil {
		log.Error(err, "unable to check Repo")
		return
	}
	restoreSession := formolv1alpha1.RestoreSession{}
	if err := session.Get(session.Context, client.ObjectKey{
		Name:      restoreSessionName,
		Namespace: restoreSessionNamespace,
	}, &restoreSession); err != nil {
		log.Error(err, "unable to get restoresession", "name", restoreSessionName, "namespace", restoreSessionNamespace)
		return
	}
	backupSession := formolv1alpha1.BackupSession{
		Spec:   restoreSession.Spec.BackupSessionRef.Spec,
		Status: restoreSession.Spec.BackupSessionRef.Status,
	}
	for i, target := range backupSession.Status.Targets {
		if target.TargetName == targetName {

			log.V(0).Info("StartRestore called", "restoring snapshot", target.SnapshotId)
			cmd := exec.Command(controllers.RESTIC_EXEC, "restore", target.SnapshotId, "--target", "/")
			// the restic restore command does not support JSON output
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Error(err, "unable to restore snapshot", "output", output)
				restoreSession.Status.Targets[i].SessionState = formolv1alpha1.Failure
			} else {
				restoreSession.Status.Targets[i].SessionState = formolv1alpha1.Waiting
				log.V(0).Info("restore was a success. Moving to waiting state", "target", target.TargetName)
			}
			if err := session.Status().Update(session.Context, &restoreSession); err != nil {
				log.Error(err, "unable to update RestoreSession", "restoreSession", restoreSession)
				return
			}
			log.V(0).Info("restore over. removing the initContainer")
			targetObject, targetPodSpec := formolv1alpha1.GetTargetObjects(target.TargetKind)
			if err := session.Get(session.Context, client.ObjectKey{
				Namespace: restoreSessionNamespace,
				Name:      target.TargetName,
			}, targetObject); err != nil {
				log.Error(err, "unable to get target objects", "target", target.TargetName)
				return
			}
			initContainers := []corev1.Container{}
			for _, c := range targetPodSpec.InitContainers {
				if c.Name == formolv1alpha1.RESTORECONTAINER_NAME {
					continue
				}
				initContainers = append(initContainers, c)
			}
			targetPodSpec.InitContainers = initContainers
			if err := session.Update(session.Context, targetObject); err != nil {
				log.Error(err, "unable to remove the restore initContainer", "targetObject", targetObject)
				return
			}
			break
		}
	}
}

func CreateBackupSession(ref corev1.ObjectReference) {
	log := session.Log.WithName("CreateBackupSession")
	log.V(0).Info("CreateBackupSession called")
	backupConf := formolv1alpha1.BackupConfiguration{}
	if err := session.Get(session.Context, types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}, &backupConf); err != nil {
		log.Error(err, "unable to get backupconf")
		os.Exit(1)
	}
	log.V(0).Info("got backupConf", "backupConf", backupConf)

	backupSession := &formolv1alpha1.BackupSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{BACKUPSESSION_PREFIX, ref.Name, strconv.FormatInt(time.Now().Unix(), 10)}, "-"),
			Namespace: ref.Namespace,
		},
		Spec: formolv1alpha1.BackupSessionSpec{
			Ref: ref,
		},
	}
	log.V(1).Info("create backupsession", "backupSession", backupSession)
	if err := session.Create(session.Context, backupSession); err != nil {
		log.Error(err, "unable to create backupsession")
		os.Exit(1)
	}
}

func DeleteSnapshot(namespace string, name string, snapshotId string) {
	log := session.Log.WithName("DeleteSnapshot")
	session.Namespace = namespace
	backupConf := formolv1alpha1.BackupConfiguration{}
	if err := session.Get(session.Context, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &backupConf); err != nil {
		log.Error(err, "unable to get the BackupConf")
		return
	}
	if err := session.SetResticEnv(backupConf); err != nil {
		log.Error(err, "unable to set the restic env")
		return
	}
	log.V(0).Info("deleting restic snapshot", "snapshotId", snapshotId)
	cmd := exec.Command(controllers.RESTIC_EXEC, "forget", "--prune", snapshotId)
	_, err := cmd.CombinedOutput()
	if err != nil {
		log.Error(err, "unable to delete snapshot", "snapshoId", snapshotId)
	}
}
