package controllers

import (
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *BackupSessionReconciler) getSecretData(name string) map[string][]byte {
	secret := corev1.Secret{}
	namespace := os.Getenv(formolv1alpha1.POD_NAMESPACE)
	if err := r.Get(r.Context, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &secret); err != nil {
		r.Log.Error(err, "unable to get Secret", "Secret", name)
		return nil
	}
	return secret.Data
}

func (r *BackupSessionReconciler) getEnvFromSecretKeyRef(name string, key string) string {
	if data := r.getSecretData(name); data != nil {
		return string(data[key])
	}
	return ""
}

func (r *BackupSessionReconciler) getConfigMapData(name string) map[string]string {
	configMap := corev1.ConfigMap{}
	namespace := os.Getenv(formolv1alpha1.POD_NAMESPACE)
	if err := r.Get(r.Context, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &configMap); err != nil {
		r.Log.Error(err, "unable to get ConfigMap", "configmap", name)
		return nil
	}
	return configMap.Data
}

func (r *BackupSessionReconciler) getEnvFromConfigMapKeyRef(name string, key string) string {
	if data := r.getConfigMapData(name); data != nil {
		return string(data[key])
	}
	return ""
}

func (r *BackupSessionReconciler) getFuncEnv(vars map[string]string, envVars []corev1.EnvVar) {
	for _, env := range envVars {
		if env.ValueFrom != nil {
			if env.ValueFrom.ConfigMapKeyRef != nil {
				vars[env.Name] = r.getEnvFromConfigMapKeyRef(env.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, env.ValueFrom.ConfigMapKeyRef.Key)
			}
			if env.ValueFrom.SecretKeyRef != nil {
				vars[env.Name] = r.getEnvFromSecretKeyRef(env.ValueFrom.SecretKeyRef.LocalObjectReference.Name, env.ValueFrom.SecretKeyRef.Key)
			}
		}
	}
}

func (r *BackupSessionReconciler) getEnvFromSecretEnvSource(vars map[string]string, name string) {
	for key, value := range r.getSecretData(name) {
		vars[key] = string(value)
	}
}

func (r *BackupSessionReconciler) getEnvFromConfigMapEnvSource(vars map[string]string, name string) {
	for key, value := range r.getConfigMapData(name) {
		vars[key] = value
	}
}

func (r *BackupSessionReconciler) getFuncEnvFrom(vars map[string]string, envVars []corev1.EnvFromSource) {
	for _, env := range envVars {
		if env.ConfigMapRef != nil {
			r.getEnvFromConfigMapEnvSource(vars, env.ConfigMapRef.LocalObjectReference.Name)
		}
		if env.SecretRef != nil {
			r.getEnvFromSecretEnvSource(vars, env.SecretRef.LocalObjectReference.Name)
		}
	}
}

func (r *BackupSessionReconciler) getFuncVars(function formolv1alpha1.Function, vars map[string]string) {
	r.getFuncEnvFrom(vars, function.Spec.EnvFrom)
	r.getFuncEnv(vars, function.Spec.Env)
}

func (r *BackupSessionReconciler) runInitBackupSteps(target formolv1alpha1.Target) error {
	namespace := os.Getenv(formolv1alpha1.POD_NAMESPACE)
	r.Log.V(0).Info("start to run the backup initializing steps it any")
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
			vars := make(map[string]string)
			r.getFuncVars(function, vars)

		}
	}
	return nil
}
