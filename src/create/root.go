package create

import (
	"fmt"
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
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
)

func CreateBackupSession(name string, namespace string) {
	fmt.Println("CreateBackupSession called")
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config",))
		if err != nil {
			panic(err.Error())
		}
	}
	scheme := runtime.NewScheme()
	_ = formolv1alpha1.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)
	cl, err := client.New(config, client.Options{Scheme: scheme})

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
	if err != nil {
		panic(err.Error())
	}
	if err := cl.Create(context.TODO(), backupSession); err != nil {
		panic(err.Error())
	}
}
