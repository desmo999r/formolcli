package create

import (
	"fmt"
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	formolv1alpha1 "formol.desmojim.fr/api/v1alpha1"
)

func CreateBackupSession(name string, namespace string) {
	fmt.Println("CreateBackupSession called")
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))
	backupSession := &formolv1alpha1.BackupSession{
		Name: name,
		Namespace: namespace,
	}
	if err := clientset.Create(context.TODO(), backupSession); err != nil {
		panic(err.Error())
	}
}
