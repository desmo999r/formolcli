package backup

import (
	"fmt"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/desmo999r/formolcli/pkg/restic"
	"github.com/desmo999r/formolcli/pkg/session"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"os/exec"
)

var (
	pg_dumpExec = "/usr/bin/pg_dump"
	logger      logr.Logger
)

func init() {
	zapLog, _ := zap.NewDevelopment()
	logger = zapr.NewLogger(zapLog)
}

func BackupVolume(tag string, paths []string) error {
	log := logger.WithName("backup-volume")
	state := formolv1alpha1.Success
	output, err := restic.BackupPaths(tag, paths)
	var snapshotId string
	if err != nil {
		log.Error(err, "unable to backup volume", "output", string(output))
		state = formolv1alpha1.Failure
	} else {
		snapshotId = restic.GetBackupResults(output)
	}
	session.BackupSessionUpdateTargetStatus(state, snapshotId)
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
	cmd := exec.Command(pg_dumpExec, "--format=custom", "--clean", "--create", "--file", file, "--host", hostname, "--dbname", database, "--username", username, "--no-password")
	cmd.Env = append(os.Environ(), "PGPASSFILE=/output/.pgpass")
	output, err := cmd.CombinedOutput()
	log.V(1).Info("postgres backup output", "output", string(output))
	if err != nil {
		log.Error(err, "something went wrong during the backup")
		return err
	}
	return nil
}
