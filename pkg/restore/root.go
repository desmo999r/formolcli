package restore

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
	//psqlExec = "/usr/bin/psql"
	pg_restoreExec = "/usr/bin/pg_restore"
	logger         logr.Logger
)

func init() {
	zapLog, _ := zap.NewDevelopment()
	logger = zapr.NewLogger(zapLog)
}

func RestoreVolume(snapshotId string) error {
	log := logger.WithName("restore-volume")
	if err := session.RestoreSessionUpdateTargetStatus(formolv1alpha1.Init); err != nil {
		return err
	}
	state := formolv1alpha1.Finalize
	output, err := restic.RestorePaths(snapshotId)
	if err != nil {
		log.Error(err, "unable to restore volume", "output", string(output))
		state = formolv1alpha1.Failure
	}
	log.V(1).Info("restic restore output", "output", string(output))
	session.RestoreSessionUpdateTargetStatus(state)
	return err
}

func RestorePostgres(file string, hostname string, database string, username string, password string) error {
	log := logger.WithName("restore-postgres")
	pgpass := []byte(fmt.Sprintf("%s:*:%s:%s:%s", hostname, database, username, password))
	if err := ioutil.WriteFile("/output/.pgpass", pgpass, 0600); err != nil {
		log.Error(err, "unable to write password to /output/.pgpass")
		return err
	}
	defer os.Remove("/output/.pgpass")
	//cmd := exec.Command(psqlExec, "--file", file, "--host", hostname, "--dbname", database, "--username", username, "--no-password")
	cmd := exec.Command(pg_restoreExec, "--format=custom", "--clean", "--host", hostname, "--dbname", database, "--username", username, "--no-password", file)
	cmd.Env = append(os.Environ(), "PGPASSFILE=/output/.pgpass")
	output, err := cmd.CombinedOutput()
	log.V(1).Info("postgres restore output", "output", string(output))
	if err != nil {
		log.Error(err, "something went wrong during the restore")
		return err
	}
	return nil
}
