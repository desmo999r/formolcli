package backupsession

import (
	"context"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	ctx    context.Context
)

func init() {
	logger = zap.New(zap.UseDevMode(true))
	ctx = context.Background()
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

func CreateBackupSession(ref corev1.ObjectReference) {
	log := logger.WithName("CreateBackupSession")
	log.V(0).Info("CreateBackupSession called")
	backupConf := formolv1alpha1.BackupConfiguration{}
	if err := cl.Get(ctx, types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}, &backupConf); err != nil {
		log.Error(err, "unable to get backupconf")
		os.Exit(1)
	}
	log.V(0).Info("got backupConf", "backupConf", backupConf)

	backupSession := &formolv1alpha1.BackupSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{"backupsession", ref.Name, strconv.FormatInt(time.Now().Unix(), 10)}, "-"),
			Namespace: ref.Namespace,
		},
		Spec: formolv1alpha1.BackupSessionSpec{
			Ref: ref,
		},
	}
	log.V(1).Info("create backupsession", "backupSession", backupSession)
	if err := cl.Create(ctx, backupSession); err != nil {
		log.Error(err, "unable to create backupsession")
		os.Exit(1)
	}
}
