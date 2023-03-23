package controllers

import (
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *RestoreSessionReconciler) restoreInitContainer(target formolv1alpha1.Target) error {
	// The restore has to be done by an initContainer since the data is mounted RO
	// We create the initContainer here
	// Once the the container has rebooted and the initContainer has done its job, it will change the restoreTargetStatus to Waiting.
	targetObject, targetPodSpec := formolv1alpha1.GetTargetObjects(target.TargetKind)
	if err := r.Get(r.Context, client.ObjectKey{
		Namespace: r.backupConf.Namespace,
		Name:      target.TargetName,
	}, targetObject); err != nil {
		r.Log.Error(err, "unable to get target objects", "target", target.TargetName)
		return err
	}
	initContainer := corev1.Container{}
	for _, c := range targetPodSpec.Containers {
		if c.Name == formolv1alpha1.SIDECARCONTAINER_NAME {
			// We copy the existing formol sidecar container to keep the VolumeMounts
			// We just have to change the name
			// Change the VolumeMounts to RW
			// Change the command so the initContainer restores the snapshot
			c.DeepCopyInto(&initContainer)
			break
		}
	}
	initContainer.Name = formolv1alpha1.RESTORECONTAINER_NAME
	for i, _ := range initContainer.VolumeMounts {
		initContainer.VolumeMounts[i].ReadOnly = false
	}
	if env, err := r.getResticEnv(r.backupConf); err != nil {
		r.Log.Error(err, "unable to get restic env")
		return err
	} else {
		initContainer.Env = append(initContainer.Env, env...)
	}
	initContainer.Args = []string{"restoresession", "start",
		"--name", r.restoreSession.Name,
		"--namespace", r.restoreSession.Namespace,
		"--target-name", target.TargetName,
	}
	targetPodSpec.InitContainers = append(targetPodSpec.InitContainers, initContainer)
	// This will kill this Pod and start a new one with the initContainer
	// the initContainer will restore the snapshot
	// If everything goes well the initContainer will change the restoreTargetStatus to Waiting
	if err := r.Update(r.Context, targetObject); err != nil {
		r.Log.Error(err, "unable to add the restore init container", "targetObject", targetObject)
		return err
	}
	return nil
}
