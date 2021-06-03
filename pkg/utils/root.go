package utils

import (
	"bytes"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
)

var logger logr.Logger

func init() {
	zapLog, _ := zap.NewDevelopment()
	logger = zapr.NewLogger(zapLog)
}

func Run(runCmd string, args []string) error {
	log := logger.WithValues("Run", runCmd, "Args", args)
	cmd := exec.Command(runCmd, args...)
	output, err := cmd.CombinedOutput()
	log.V(1).Info("result", "output", string(output))
	if err != nil {
		log.Error(err, "something went wrong")
		return err
	}
	return nil
}

func RunChroot(lookForTag bool, runCmd string, args ...string) error {
	log := logger.WithValues("RunChroot", runCmd, "Args", args)
	root := regexp.MustCompile(`/proc/[0-9]+/root`)
	env := regexp.MustCompile(`/proc/[0-9]+/environ`)
	pid := strconv.Itoa(os.Getpid())
	skip := false
	if err := filepath.Walk("/proc", func(path string, info os.FileInfo, err error) error {
		if skip {
			return filepath.SkipDir
		}
		if err != nil {
			return nil
		}
		if info.IsDir() && (info.Name() == "1" || info.Name() == pid) {
			return filepath.SkipDir
		}
		if lookForTag && env.MatchString(path) {
			log.V(0).Info("Looking for tag", "file", path)
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return filepath.SkipDir
			}

			var matched bool
			for _, envVar := range bytes.Split(content, []byte{'\000'}) {
				matched, err = regexp.Match(formolv1alpha1.TARGETCONTAINER_TAG, envVar)
				if err != nil {
					log.Error(err, "cannot regexp")
					return err
				}
				if matched {
					log.V(0).Info("Found the target tag", "file", path)
					break
				}
			}
			if matched == false {
				return filepath.SkipDir
			}
		}
		if root.MatchString(path) {
			if _, err := filepath.EvalSymlinks(path); err != nil {
				return filepath.SkipDir
			}
			log.V(0).Info("running chroot in", "path", path)
			cmd := exec.Command("chroot", append([]string{path, runCmd}, args...)...)
			output, err := cmd.CombinedOutput()
			log.V(0).Info("result", "output", string(output))
			if err != nil {
				log.Error(err, "something went wrong")
				return err
			}
			skip = true
			return filepath.SkipDir
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
