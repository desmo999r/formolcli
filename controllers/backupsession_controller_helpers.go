package controllers

import (
	"bufio"
	"encoding/json"
	"fmt"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"io"
	"os"
	"os/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

const (
	RESTIC_EXEC = "/usr/bin/restic"
)

func (r *BackupSessionReconciler) setResticEnv(backupConf formolv1alpha1.BackupConfiguration) error {
	repo := formolv1alpha1.Repo{}
	if err := r.Get(r.Context, client.ObjectKey{
		Namespace: backupConf.Namespace,
		Name:      backupConf.Spec.Repository,
	}, &repo); err != nil {
		r.Log.Error(err, "unable to get repo")
		return err
	}
	if repo.Spec.Backend.S3 != nil {
		os.Setenv(formolv1alpha1.RESTIC_REPOSITORY, fmt.Sprintf("s3:http://%s/%s/%s-%s",
			repo.Spec.Backend.S3.Server,
			repo.Spec.Backend.S3.Bucket,
			strings.ToUpper(backupConf.Namespace),
			strings.ToLower(backupConf.Name)))
		data := r.getSecretData(repo.Spec.RepositorySecrets)
		os.Setenv(formolv1alpha1.AWS_SECRET_ACCESS_KEY, string(data[formolv1alpha1.AWS_SECRET_ACCESS_KEY]))
		os.Setenv(formolv1alpha1.AWS_ACCESS_KEY_ID, string(data[formolv1alpha1.AWS_ACCESS_KEY_ID]))
		os.Setenv(formolv1alpha1.RESTIC_PASSWORD, string(data[formolv1alpha1.RESTIC_PASSWORD]))
	}
	return nil
}

func (r *BackupSessionReconciler) checkRepo() error {
	r.Log.V(0).Info("Checking repo")
	if err := exec.Command(RESTIC_EXEC, "unlock").Run(); err != nil {
		r.Log.Error(err, "unable to unlock repo", "repo", os.Getenv(formolv1alpha1.RESTIC_REPOSITORY))
	}
	output, err := exec.Command(RESTIC_EXEC, "check").CombinedOutput()
	if err != nil {
		r.Log.V(0).Info("Initializing new repo")
		output, err = exec.Command(RESTIC_EXEC, "init").CombinedOutput()
		if err != nil {
			r.Log.Error(err, "something went wrong during repo init", "output", output)
		}
	}
	return err
}

type BackupResult struct {
	SnapshotId string
	Duration   float64
}

func (r *BackupSessionReconciler) backupPaths(tag string, paths []string) (result BackupResult, err error) {
	if err = r.checkRepo(); err != nil {
		r.Log.Error(err, "unable to setup repo", "repo", os.Getenv(formolv1alpha1.RESTIC_REPOSITORY))
		return
	}
	r.Log.V(0).Info("backing up paths", "paths", paths)
	cmd := exec.Command(RESTIC_EXEC, append([]string{"backup", "--json", "--tag", tag}, paths...)...)
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

func (r *BackupSessionReconciler) backupJob(tag string, target formolv1alpha1.Target) (result BackupResult, err error) {
	paths := []string{}
	for _, container := range target.Containers {
		for _, job := range container.Job {
			if err = r.runFunction(job.Name); err != nil {
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
	result, err = r.backupPaths(tag, paths)
	return
}
