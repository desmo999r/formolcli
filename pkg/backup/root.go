package backup

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/desmo999r/formolcli/pkg/backupsession"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"io/ioutil"
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
	pg_dumpExec           = "/usr/bin/pg_dump"
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
	log := logger.WithName("backup-checkrepo")
	cmd := exec.Command(resticExec, "check", "-r", repo)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Error(err, "unable to pipe stderr")
		return err
	}
	if err := cmd.Start(); err != nil {
		log.Error(err, "cannot start repo check")
		return err
	}
	if err := cmd.Wait(); err != nil {
		log.V(0).Info("initializing new repo", "repo", repo)
		cmd = exec.Command(resticExec, "init", "-r", repo)
		if err := cmd.Start(); err != nil {
			log.Error(err, "cannot start repo init")
			return err
		}
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				log.V(0).Info("and error happened", "stderr", scanner.Text())
			}
		}()
		if err := cmd.Wait(); err != nil {
			log.Error(err, "something went wrong during repo init")
			return err
		}
	}
	return err
}

func GetBackupResults(output []byte) (snapshotId string, duration time.Duration) {
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
			duration = time.Duration(dat["total_duration"].(float64)*1000) * time.Millisecond
		}
	}
	return
}

func BackupVolume(tag string, paths []string) error {
	log := logger.WithName("backup-volume")
	state := formolv1alpha1.Success
	output, err := BackupPaths(tag, paths)
	var snapshotId string
	var duration time.Duration
	if err != nil {
		log.Error(err, "unable to backup volume", "output", string(output))
		state = formolv1alpha1.Failure
	} else {
		snapshotId, duration = GetBackupResults(output)
	}
	backupsession.BackupSessionUpdateStatus(state, snapshotId, duration)
	return err
}

func BackupPostgres(file string, hostname string, database string, username string, password string) error {
	log := logger.WithName("backup-postgres")
	pgpass := []byte(fmt.Sprintf("%s:*:%s:%s:%s", hostname, database, username, password))
	if err := ioutil.WriteFile("/output/.pgpass", pgpass, 0600); err != nil {
		log.Error(err, "unable to write password to /output/.pgpass")
		return err
	}
	defer os.Remove("/output/.pgpass")
	cmd := exec.Command(pg_dumpExec, "--clean", "--create", "--file", file, "--host", hostname, "--dbname", database, "--username", username, "--no-password")
	cmd.Env = append(os.Environ(), "PGPASSFILE=/output/.pgpass")
	output, err := cmd.CombinedOutput()
	log.V(1).Info("postgres backup output", "output", string(output))
	if err != nil {
		log.Error(err, "something went wrong during the backup")
		return err
	}
	return nil
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
