package controllers

import (
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

const (
	JOBTTL int32 = 7200
)

func (r *BackupSessionReconciler) backupJob(target formolv1alpha1.Target) (result BackupResult, err error) {
	paths := []string{}
	for _, container := range target.Containers {
		for _, job := range container.Job {
			if err = r.runFunction(*job.Backup); err != nil {
				r.Log.Error(err, "unable to run job")
				return
			}

		}
		addPath := true
		for _, path := range paths {
			if path == container.SharePath {
				addPath = false
			}
		}
		if addPath {
			paths = append(paths, container.SharePath)
		}
	}
	result, err = r.BackupPaths(paths)
	return
}

func (r *BackupSessionReconciler) backupSnapshot(target formolv1alpha1.Target) (e error) {
	targetObject, targetPodSpec := formolv1alpha1.GetTargetObjects(target.TargetKind)
	if err := r.Get(r.Context, client.ObjectKey{
		Namespace: r.Namespace,
		Name:      target.TargetName,
	}, targetObject); err != nil {
		r.Log.Error(err, "cannot get target", "target", target.TargetName)
		return err
	}
	for _, container := range targetPodSpec.Containers {
		for _, targetContainer := range target.Containers {
			if targetContainer.Name == container.Name {
				// Now snapshot all the container PVC that support snapshots
				// then create new volumes from the snapshots
				// replace the volumes in the container struct with the snapshot volumes
				// use formolv1alpha1.GetVolumeMounts to get the volume mounts for the Job
				// sidecar := formolv1alpha1.GetSidecar(backupConf, target)
				paths, vms := formolv1alpha1.GetVolumeMounts(container, targetContainer)
				if err := r.snapshotVolumes(vms, targetPodSpec); err != nil {
					if IsNotReadyToUse(err) {
						r.Log.V(0).Info("Some volumes are still not ready to use")
						defer func() { e = &NotReadyToUseError{} }()
					} else {
						r.Log.Error(err, "cannot snapshot the volumes")
						return err
					}
				} else {
					r.Log.V(1).Info("Creating a Job to backup the Snapshot volumes")
					sidecar := formolv1alpha1.GetSidecar(r.backupConf, target)
					sidecar.Args = append([]string{"backupsession", "backup", "--namespace", r.Namespace, "--name", r.Name, "--target-name", target.TargetName}, paths...)
					sidecar.VolumeMounts = vms
					if env, err := r.getResticEnv(r.backupConf); err != nil {
						r.Log.Error(err, "unable to get restic env")
						return err
					} else {
						sidecar.Env = append(sidecar.Env, env...)
					}
					sidecar.Env = append(sidecar.Env, corev1.EnvVar{
						Name:  formolv1alpha1.BACKUP_PATHS,
						Value: strings.Join(paths, string(os.PathListSeparator)),
					})
					job := batchv1.Job{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: r.Namespace,
							Name:      "backupsnapshot-" + r.Name,
						},
						Spec: batchv1.JobSpec{
							TTLSecondsAfterFinished: func() *int32 { ttl := JOBTTL; return &ttl }(),
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Volumes: targetPodSpec.Volumes,
									Containers: []corev1.Container{
										sidecar,
									},
									RestartPolicy: corev1.RestartPolicyNever,
								},
							},
						},
					}
					if err := r.Create(r.Context, &job); err != nil {
						r.Log.Error(err, "unable to create the snapshot volumes backup job", "job", job, "container", sidecar)
						return err
					}
					r.Log.V(1).Info("snapshot volumes backup job created", "job", job.Name)
				}
			}
		}
	}
	return nil
}

type NotReadyToUseError struct{}

func (e *NotReadyToUseError) Error() string {
	return "Snapshot is not ready to use"
}

func IsNotReadyToUse(err error) bool {
	switch err.(type) {
	case *NotReadyToUseError:
		return true
	default:
		return false
	}
}

func (r *BackupSessionReconciler) snapshotVolume(volume corev1.Volume) (*volumesnapshotv1.VolumeSnapshot, error) {
	r.Log.V(0).Info("Preparing snapshot", "volume", volume.Name)
	if volume.VolumeSource.PersistentVolumeClaim != nil {
		pvc := corev1.PersistentVolumeClaim{}
		if err := r.Get(r.Context, client.ObjectKey{
			Namespace: r.Namespace,
			Name:      volume.VolumeSource.PersistentVolumeClaim.ClaimName,
		}, &pvc); err != nil {
			r.Log.Error(err, "unable to get pvc", "volume", volume)
			return nil, err
		}
		pv := corev1.PersistentVolume{}
		if err := r.Get(r.Context, client.ObjectKey{
			Name: pvc.Spec.VolumeName,
		}, &pv); err != nil {
			r.Log.Error(err, "unable to get pv", "volume", pvc.Spec.VolumeName)
			return nil, err
		}
		if pv.Spec.PersistentVolumeSource.CSI != nil {
			// This volume is supported by a CSI driver. Let's see if we can snapshot it.
			volumeSnapshotClassList := volumesnapshotv1.VolumeSnapshotClassList{}
			if err := r.List(r.Context, &volumeSnapshotClassList); err != nil {
				r.Log.Error(err, "unable to get VolumeSnapshotClass list")
				return nil, err
			}
			for _, volumeSnapshotClass := range volumeSnapshotClassList.Items {
				if volumeSnapshotClass.Driver == pv.Spec.PersistentVolumeSource.CSI.Driver {
					// Check if a snapshot exist
					volumeSnapshot := volumesnapshotv1.VolumeSnapshot{}
					volumeSnapshotName := strings.Join([]string{"vs", r.Name, pv.Name}, "-")

					if err := r.Get(r.Context, client.ObjectKey{
						Namespace: r.Namespace,
						Name:      volumeSnapshotName,
					}, &volumeSnapshot); errors.IsNotFound(err) {
						// No snapshot found. Create a new one.
						// We want to snapshot using this VolumeSnapshotClass
						r.Log.V(0).Info("Create a volume snapshot", "pvc", pvc.Name)
						volumeSnapshot = volumesnapshotv1.VolumeSnapshot{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: r.Namespace,
								Name:      volumeSnapshotName,
								Labels: map[string]string{
									"backupsession": r.Name,
								},
							},
							Spec: volumesnapshotv1.VolumeSnapshotSpec{
								VolumeSnapshotClassName: &volumeSnapshotClass.Name,
								Source: volumesnapshotv1.VolumeSnapshotSource{
									PersistentVolumeClaimName: &pvc.Name,
								},
							},
						}
						if err := r.Create(r.Context, &volumeSnapshot); err != nil {
							r.Log.Error(err, "unable to create the snapshot", "pvc", pvc.Name)
							return nil, err
						}
						// We just created the snapshot. We have to assume it's not yet ready and reschedule
						return nil, &NotReadyToUseError{}
					} else {
						if err != nil {
							r.Log.Error(err, "Something went very wrong here")
							return nil, err
						}
						// The VolumeSnapshot exists. Is it ReadyToUse?
						if volumeSnapshot.Status == nil || volumeSnapshot.Status.ReadyToUse == nil || *volumeSnapshot.Status.ReadyToUse == false {
							r.Log.V(0).Info("Volume snapshot exists but it is not ready", "volume", volumeSnapshot.Name)
							return nil, &NotReadyToUseError{}
						}
						r.Log.V(0).Info("Volume snapshot is ready to use", "volume", volumeSnapshot.Name)
						return &volumeSnapshot, nil
					}
				}
			}
		}
	}
	return nil, nil
}

func (r *BackupSessionReconciler) createVolumeFromSnapshot(vs *volumesnapshotv1.VolumeSnapshot) (backupPVCName string, err error) {
	backupPVCName = strings.Replace(vs.Name, "vs", "bak", 1)
	backupPVC := corev1.PersistentVolumeClaim{}
	if err = r.Get(r.Context, client.ObjectKey{
		Namespace: r.Namespace,
		Name:      backupPVCName,
	}, &backupPVC); errors.IsNotFound(err) {
		// The Volume does not exist. Create it.
		pv := corev1.PersistentVolume{}
		pvName, _ := strings.CutPrefix(vs.Name, strings.Join([]string{"vs", r.Name}, "-"))
		pvName = pvName[1:]
		if err = r.Get(r.Context, client.ObjectKey{
			Name: pvName,
		}, &pv); err != nil {
			r.Log.Error(err, "unable to find pv", "pv", pvName)
			return
		}
		backupPVC = corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: r.Namespace,
				Name:      backupPVCName,
				Labels: map[string]string{
					"backupsession": r.Name,
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &pv.Spec.StorageClassName,
				//AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				AccessModes: pv.Spec.AccessModes,
				Resources: corev1.ResourceRequirements{
					Requests: pv.Spec.Capacity,
				},
				DataSource: &corev1.TypedLocalObjectReference{
					APIGroup: func() *string { s := "snapshot.storage.k8s.io"; return &s }(),
					Kind:     "VolumeSnapshot",
					Name:     vs.Name,
				},
			},
		}
		if err = r.Create(r.Context, &backupPVC); err != nil {
			r.Log.Error(err, "unable to create backup PVC", "backupPVC", backupPVC)
			return
		}
	}
	if err != nil {
		r.Log.Error(err, "something went very wrong here")
	}
	return
}

func (r *BackupSessionReconciler) snapshotVolumes(vms []corev1.VolumeMount, podSpec *corev1.PodSpec) (err error) {
	// We snapshot/check all the volumes. If at least one of the snapshot is not ready to use. We reschedule.
	for _, vm := range vms {
		for i, volume := range podSpec.Volumes {
			if vm.Name == volume.Name {
				var vs *volumesnapshotv1.VolumeSnapshot
				vs, err = r.snapshotVolume(volume)
				if IsNotReadyToUse(err) {
					defer func() {
						err = &NotReadyToUseError{}
					}()
					continue
				}
				if err != nil {
					return
				}
				if vs != nil {
					// The snapshot is ready. We create a PVC from it.
					backupPVCName, err := r.createVolumeFromSnapshot(vs)
					if err != nil {
						r.Log.Error(err, "unable to create volume from snapshot", "vs", vs)
						return err
					}
					podSpec.Volumes[i].VolumeSource.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: backupPVCName,
						ReadOnly:  true,
					}
					// The snapshot and the volume will be deleted by the Job when the backup is over
				}
			}
		}
	}
	return
}
