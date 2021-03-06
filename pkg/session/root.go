package session

import (
	"context"
	"errors"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strconv"
	"strings"
	"time"
)

var (
	config *rest.Config
	scheme *runtime.Scheme
	cl     client.Client
	logger logr.Logger
	//backupSession *formolv1alpha1.BackupSession
)

func init() {
	logger = zap.New(zap.UseDevMode(true))
	log := logger.WithName("InitBackupSession")
	ctrl.SetLogger(logger)
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			log.Error(err, "unable to get config")
			os.Exit(1)
		}
	}
	scheme = runtime.NewScheme()
	_ = formolv1alpha1.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)
	cl, err = client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		log.Error(err, "unable to get client")
		os.Exit(1)
	}
}

func BackupSessionUpdateTargetStatus(state formolv1alpha1.SessionState, snapshotId string) error {
	log := logger.WithName("BackupSessionUpdateStatus")
	targetName := os.Getenv("TARGET_NAME")
	backupSession := &formolv1alpha1.BackupSession{}
	if err := cl.Get(context.Background(), client.ObjectKey{
		Namespace: os.Getenv("BACKUPSESSION_NAMESPACE"),
		Name:      os.Getenv("BACKUPSESSION_NAME"),
	}, backupSession); err != nil {
		log.Error(err, "unable to get backupsession", "BACKUPSESSION_NAME", os.Getenv("BACKUPSESSION_NAME"), "BACKUPSESSION_NAMESPACE", os.Getenv("BACKUPSESSION_NAMESPACE"))
		return err
	}
	for i, target := range backupSession.Status.Targets {
		if target.Name == targetName {
			backupSession.Status.Targets[i].SessionState = state
			backupSession.Status.Targets[i].SnapshotId = snapshotId
			backupSession.Status.Targets[i].Duration = &metav1.Duration{Duration: time.Now().Sub(backupSession.Status.Targets[i].StartTime.Time)}
		}
	}

	if err := cl.Status().Update(context.Background(), backupSession); err != nil {
		log.Error(err, "unable to update status", "backupsession", backupSession)
		return err
	}
	return nil
}

func RestoreSessionUpdateTargetStatus(state formolv1alpha1.SessionState) error {
	log := logger.WithName("RestoreSessionUpdateStatus")
	targetName := os.Getenv("TARGET_NAME")
	restoreSession := &formolv1alpha1.RestoreSession{}
	if err := cl.Get(context.Background(), client.ObjectKey{
		Namespace: os.Getenv("RESTORESESSION_NAMESPACE"),
		Name:      os.Getenv("RESTORESESSION_NAME"),
	}, restoreSession); err != nil {
		log.Error(err, "unable to get backupsession", "RESTORESESSION_NAME", os.Getenv("RESTORESESSION_NAME"), "RESTORESESSION_NAMESPACE", os.Getenv("RESTORESESSION_NAMESPACE"))
		return err
	}
	for i, target := range restoreSession.Status.Targets {
		if target.Name == targetName {
			if target.SessionState == formolv1alpha1.Success {
				return errors.New("the restore has already been done. Skipping")
			}
			restoreSession.Status.Targets[i].SessionState = state
			if state == formolv1alpha1.Success || state == formolv1alpha1.Failure {
				restoreSession.Status.Targets[i].Duration = &metav1.Duration{Duration: time.Now().Sub(restoreSession.Status.Targets[i].StartTime.Time)}
			}
		}
	}

	if err := cl.Status().Update(context.Background(), restoreSession); err != nil {
		log.Error(err, "unable to update restoresession status", "restoresession", restoreSession)
		return err
	}
	return nil
}

func CreateBackupSession(name string, namespace string) {
	log := logger.WithName("CreateBackupSession")
	log.V(0).Info("CreateBackupSession called")
	backupConfList := &formolv1alpha1.BackupConfigurationList{}
	if err := cl.List(context.TODO(), backupConfList, client.InNamespace(namespace)); err != nil {
		log.Error(err, "unable to get backupconf")
		os.Exit(1)
	}
	backupConf := &formolv1alpha1.BackupConfiguration{}
	for _, bc := range backupConfList.Items {
		if bc.Name == name {
			*backupConf = bc
		}
	}
	log.V(0).Info("got backupConf", "backupConf", backupConf)

	backupSession := &formolv1alpha1.BackupSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{"backupsession", name, strconv.FormatInt(time.Now().Unix(), 10)}, "-"),
			Namespace: namespace,
		},
		Spec: formolv1alpha1.BackupSessionSpec{
			Ref: formolv1alpha1.Ref{
				Name: name,
			},
		},
	}
	log.V(1).Info("create backupsession", "backupSession", backupSession)
	if err := cl.Create(context.TODO(), backupSession); err != nil {
		log.Error(err, "unable to create backupsession")
		os.Exit(1)
	}
}
