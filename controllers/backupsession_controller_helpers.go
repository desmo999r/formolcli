package controllers

import (
	"bufio"
	"encoding/json"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"io"
	"os"
	"os/exec"
)

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
