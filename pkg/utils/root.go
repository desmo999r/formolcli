package utils

import (
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
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

func RunHooks(hooks []formolv1alpha1.Hook) error {
	for _, hook := range hooks {
		err := RunChroot(hook.Cmd, hook.Args...)
		if err != nil {
			return err
		}
	}
	return nil
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

func RunChroot(runCmd string, args ...string) error {
	log := logger.WithValues("RunChroot", runCmd, "Args", args)
	root := regexp.MustCompile(`/proc/[0-9]+/root`)
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
		if root.MatchString(path) {
			if _, err := filepath.EvalSymlinks(path); err != nil {
				return filepath.SkipDir
			}
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
