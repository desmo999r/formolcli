package controllers

import (
	"bufio"
	"encoding/json"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"os/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type BackupResult struct {
	SnapshotId string
	Duration   float64
}

func (r *BackupSessionReconciler) backupPaths(paths []string) (result BackupResult, err error) {
	if err = r.CheckRepo(); err != nil {
		r.Log.Error(err, "unable to setup repo", "repo", os.Getenv(formolv1alpha1.RESTIC_REPOSITORY))
		return
	}
	r.Log.V(0).Info("backing up paths", "paths", paths)
	cmd := exec.Command(RESTIC_EXEC, append([]string{"backup", "--json", "--tag", r.Name}, paths...)...)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	_ = cmd.Start()

	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	scanner.Split(bufio.ScanLines)
	var data map[string]interface{}
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &data); err != nil {
			r.Log.Error(err, "unable to unmarshal json", "data", scanner.Text())
			continue
		}
		switch data["message_type"].(string) {
		case "summary":
			result.SnapshotId = data["snapshot_id"].(string)
			result.Duration = data["total_duration"].(float64)
		case "status":
			r.Log.V(0).Info("backup running", "percent done", data["percent_done"].(float64))
		}
	}

	err = cmd.Wait()
	return
}

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
	result, err = r.backupPaths(paths)
	return
}

func (r *BackupSessionReconciler) backupSnapshot(target formolv1alpha1.Target) error {
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
				_, vms := formolv1alpha1.GetVolumeMounts(container, targetContainer)
				if err := r.snapshotVolumes(vms, targetPodSpec); err != nil {
					if IsNotReadyToUse(err) {
						r.Log.V(0).Info("Some volumes are still not ready to use")
					} else {
						r.Log.Error(err, "cannot snapshot the volumes")
						return err
					}
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
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &pv.Spec.StorageClassName,
				//AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				AccessModes: pv.Spec.AccessModes,
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
