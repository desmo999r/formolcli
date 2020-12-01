package backup

import (
	"strings"
	"os"
	"os/exec"
	"log"
)

var (
	repository string
	passwordFile string
	aws_access_key_id string
	aws_secret_access_key string
	resticExec = "/usr/local/bin/restic"
)

func init() {
	if repository = os.Getenv("RESTIC_REPOSITORY"); repository == "" {
		log.Fatal("RESTIC_REPOSITORY not set")
	}
	if passwordFile = os.Getenv("RESTIC_PASSWORD"); passwordFile == "" {
		log.Fatal("RESTIC_PASSWORD not set")
	}
	if aws_access_key_id = os.Getenv("AWS_ACCESS_KEY_ID"); aws_access_key_id == "" {
		log.Fatal("AWS_ACCESS_KEY_ID not set")
	}
	if aws_secret_access_key = os.Getenv("AWS_SECRET_ACCESS_KEY"); aws_secret_access_key == "" {
		log.Fatal("AWS_SECRET_ACCESS_KEY not set")
	}
}

func checkRepo(repo string) error {
	cmd := exec.Command(resticExec, "check", "-r", repo)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		cmd = exec.Command(resticExec, "init", "-r", repo)
		err = cmd.Run()
	}
	return err
}

func BackupVolume(path string) error {
	return nil
}

func BackupDeployment(prefix string, paths []string) error {
	newrepo := repository
	if prefix != "" {
		newrepo = repository + "/" + prefix
	}
	if err := checkRepo(newrepo); err != nil {
		log.Fatal("unable to setup newrepo", "newrepo", newrepo)
		return err
	}
	cmd := exec.Command(resticExec, "backup", "-r", newrepo, strings.Join(paths, " "))

	return cmd.Run()
}
