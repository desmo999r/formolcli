package create

import (
	"strings"
	"time"
	"context"
	"os"
	"strconv"
	"path/filepath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
)

func CreateBackupSession(name string, namespace string) {
	log := zap.New(zap.UseDevMode(true)).WithName("CreateBackupSession")
	ctrl.SetLogger(log)
	log.V(0).Info("CreateBackupSession called")
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config",))
		if err != nil {
			log.Error(err, "unable to get config")
			os.Exit(1)
		}
	}
	scheme := runtime.NewScheme()
	_ = formolv1alpha1.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)
	cl, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		log.Error(err, "unable to get client")
		os.Exit(1)
	}

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

	backupSession := &formolv1alpha1.BackupSession{
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.Join([]string{"backupsession",name,strconv.FormatInt(time.Now().Unix(), 10)}, "-"),
			Namespace: namespace,
		},
		Spec: formolv1alpha1.BackupSessionSpec{
			Ref: formolv1alpha1.Ref{
				Name: name,
			},
		},
		Status: formolv1alpha1.BackupSessionStatus{},
	}
	if err := ctrl.SetControllerReference(backupConf, backupSession, scheme); err != nil {
		log.Error(err, "unable to set controller reference")
		os.Exit(1)
	}
	if err := cl.Create(context.TODO(), backupSession); err != nil {
		log.Error(err, "unable to create backupsession")
		os.Exit(1)
	}
}
