package controllers

import (
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *BackupSessionReconciler) runInitBackupSteps(target formolv1alpha1.Target) error {
	namespace := os.Getenv(formolv1alpha1.POD_NAMESPACE)
	for _, container := range target.Containers {
		for _, step := range container.Steps {
			if step.Finalize != nil && *step.Finalize == true {
				continue
			}
			function := formolv1alpha1.Function{}
			if err := r.Get(r.Context, client.ObjectKey{
				Namespace: namespace,
				Name:      step.Name,
			}, &function); err != nil {
				r.Log.Error(err, "unable to get Function", "Function", step.Name)
				return err
			}
		}
	}
	return nil
}
