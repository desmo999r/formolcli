package backup

import (
	"strings"
	"bufio"
	"os"
	"os/exec"
	"go.uber.org/zap"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
)

var (
	repository string
	passwordFile string
	aws_access_key_id string
	aws_secret_access_key string
	resticExec = "/usr/bin/restic"
	logger logr.Logger
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
		go func(){
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

func BackupVolume(path string) error {
	return nil
}

func BackupDeployment(prefix string, paths []string, c chan []byte) (error) {
	log := logger.WithName("backup-deployment")
	newrepo := repository
	if prefix != "" {
		newrepo = repository + "/" + prefix
	}
	if err := checkRepo(newrepo); err != nil {
		log.Error(err, "unable to setup newrepo", "newrepo", newrepo)
		return err
	}
	cmd := exec.Command(resticExec, "backup", "--json", "-r", newrepo, strings.Join(paths, " "))
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Error(err, "unable to pipe stderr")
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error(err, "unable to pipe stdout")
		return err
	}
	if err := cmd.Start(); err != nil {
		log.Error(err, "cannot start backup")
		return err
	}
	go func(){
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.V(0).Info("and error happened", "stderr", scanner.Text())
		}
	}()
	go func(c chan []byte){
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			c <- scanner.Bytes()
		}
	}(c)
	if err := cmd.Wait(); err != nil {
		log.Error(err, "something went wrong during the backup")
		return err
	}

	return nil
}

func DeleteSnapshot(prefix string, snapshotId string) error {
	log := logger.WithValues("delete-snapshot", snapshotId)
	newrepo := repository
	if prefix != "" {
		newrepo = repository + "/" + prefix
	}
	cmd := exec.Command(resticExec, "forget", "-r", newrepo, snapshotId)
	if err := cmd.Run(); err != nil {
		log.Error(err, "unable to delete the snapshot")
		return err
	}
	return nil
}
