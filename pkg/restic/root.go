package restic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"os"
	"os/exec"
	"time"
)

var (
	repository            string
	passwordFile          string
	aws_access_key_id     string
	aws_secret_access_key string
	resticExec            = "/usr/bin/restic"
	logger                logr.Logger
)

func init() {
	zapLog, _ := zap.NewDevelopment()
	logger = zapr.NewLogger(zapLog)
	repository = os.Getenv("RESTIC_REPOSITORY")
	passwordFile = os.Getenv("RESTIC_PASSWORD")
	aws_access_key_id = os.Getenv("AWS_ACCESS_KEY_ID")
	aws_secret_access_key = os.Getenv("AWS_SECRET_ACCESS_KEY")
}

func checkRepo(repo string) error {
	log := logger.WithValues("backup-checkrepo", repo)
	cmd := exec.Command(resticExec, "unlock", "-r", repo)
	if err := cmd.Run(); err != nil {
		log.Error(err, "unable to unlock repo", "repo", repo)
	}
	cmd = exec.Command(resticExec, "check", "-r", repo)
	output, err := cmd.CombinedOutput()
	log.V(1).Info("restic check output", "output", string(output))
	if err != nil {
		log.V(0).Info("initializing new repo", "repo", repo)
		cmd = exec.Command(resticExec, "init", "-r", repo)
		output, err = cmd.CombinedOutput()
		log.V(1).Info("restic init repo", "output", string(output))
		if err != nil {
			log.Error(err, "something went wrong during repo init")
			return err
		}
	}
	return err
}

func GetBackupResults(output []byte) (snapshotId string) {
	log := logger.WithName("backup-getbackupresults")
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var dat map[string]interface{}
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &dat); err != nil {
			log.Error(err, "unable to unmarshal json", "msg", string(scanner.Bytes()))
			continue
		}
		log.V(1).Info("message on stdout", "stdout", dat)
		if message_type, ok := dat["message_type"]; ok && message_type == "summary" {
			snapshotId = dat["snapshot_id"].(string)
			//duration = time.Duration(dat["total_duration"].(float64)*1000) * time.Millisecond
		}
	}
	return
}

func GetRestoreResults(output []byte) time.Duration {
	return 0 * time.Millisecond
}

func BackupPaths(tag string, paths []string) ([]byte, error) {
	log := logger.WithName("backup-deployment")
	if err := checkRepo(repository); err != nil {
		log.Error(err, "unable to setup newrepo", "newrepo", repository)
		return []byte{}, err
	}
	cmd := exec.Command(resticExec, append([]string{"backup", "--json", "--tag", tag, "-r", repository}, paths...)...)
	output, err := cmd.CombinedOutput()
	return output, err
}

func RestorePaths(repository string, snapshotId string) ([]byte, error) {
	log := logger.WithName("restore-deployment")
	if err := checkRepo(repository); err != nil {
		log.Error(err, "unable to setup repo", "repo", repository)
		return []byte{}, err
	}
	cmd := exec.Command(resticExec, "restore", "-r", repository, snapshotId, "--target", "/")
	return cmd.CombinedOutput()
}

func DeleteSnapshot(snapshot string) error {
	log := logger.WithValues("delete-snapshot", snapshot)
	cmd := exec.Command(resticExec, "forget", "-r", repository, "--prune", snapshot)
	log.V(0).Info("deleting snapshot", "snapshot", snapshot)
	output, err := cmd.CombinedOutput()
	log.V(1).Info("delete snapshot output", "output", string(output))
	if err != nil {
		log.Error(err, "unable to delete the snapshot")
		return err
	}
	return nil
}
