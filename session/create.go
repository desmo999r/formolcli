package session

import (
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"strconv"
	"strings"
	"time"
)

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
