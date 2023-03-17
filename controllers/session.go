package controllers

import (
	"bufio"
	"bytes"
	"context"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/go-logr/logr"
	"io"
	"io/fs"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

type Session struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	context.Context
	Namespace string
}

func (s Session) getSecretData(name string) map[string][]byte {
	secret := corev1.Secret{}
	namespace := os.Getenv(formolv1alpha1.POD_NAMESPACE)
	if err := s.Get(s.Context, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &secret); err != nil {
		s.Log.Error(err, "unable to get Secret", "Secret", name)
		return nil
	}
	return secret.Data
}

func (s Session) getEnvFromSecretKeyRef(name string, key string) string {
	if data := s.getSecretData(name); data != nil {
		return string(data[key])
	}
	return ""
}

func (s Session) getConfigMapData(name string) map[string]string {
	configMap := corev1.ConfigMap{}
	namespace := os.Getenv(formolv1alpha1.POD_NAMESPACE)
	if err := s.Get(s.Context, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &configMap); err != nil {
		s.Log.Error(err, "unable to get ConfigMap", "configmap", name)
		return nil
	}
	return configMap.Data
}

func (s Session) getEnvFromConfigMapKeyRef(name string, key string) string {
	if data := s.getConfigMapData(name); data != nil {
		return string(data[key])
	}
	return ""
}

func (s Session) getFuncEnv(vars map[string]string, envVars []corev1.EnvVar) {
	for _, env := range envVars {
		if env.ValueFrom != nil {
			if env.ValueFrom.ConfigMapKeyRef != nil {
				vars[env.Name] = s.getEnvFromConfigMapKeyRef(env.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, env.ValueFrom.ConfigMapKeyRef.Key)
			}
			if env.ValueFrom.SecretKeyRef != nil {
				vars[env.Name] = s.getEnvFromSecretKeyRef(env.ValueFrom.SecretKeyRef.LocalObjectReference.Name, env.ValueFrom.SecretKeyRef.Key)
			}
		} else {
			vars[env.Name] = env.Value
		}
	}
}

func (s Session) getEnvFromSecretEnvSource(vars map[string]string, name string) {
	for key, value := range s.getSecretData(name) {
		vars[key] = string(value)
	}
}

func (s Session) getEnvFromConfigMapEnvSource(vars map[string]string, name string) {
	for key, value := range s.getConfigMapData(name) {
		vars[key] = value
	}
}

func (s Session) getFuncEnvFrom(vars map[string]string, envVars []corev1.EnvFromSource) {
	for _, env := range envVars {
		if env.ConfigMapRef != nil {
			s.getEnvFromConfigMapEnvSource(vars, env.ConfigMapRef.LocalObjectReference.Name)
		}
		if env.SecretRef != nil {
			s.getEnvFromSecretEnvSource(vars, env.SecretRef.LocalObjectReference.Name)
		}
	}
}

func (s Session) getFuncVars(function formolv1alpha1.Function, vars map[string]string) {
	s.getFuncEnvFrom(vars, function.Spec.EnvFrom)
	s.getFuncEnv(vars, function.Spec.Env)
}

func (s Session) runFunction(name string) error {
	namespace := os.Getenv(formolv1alpha1.POD_NAMESPACE)
	function := formolv1alpha1.Function{}
	if err := s.Get(s.Context, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &function); err != nil {
		s.Log.Error(err, "unable to get Function", "Function", name)
		return err
	}
	vars := make(map[string]string)
	s.getFuncVars(function, vars)

	s.Log.V(0).Info("function vars", "vars", vars)
	// Loop through the function.Spec.Command arguments to replace ${ARG}|$(ARG)|$ARG
	// with the environment variable value
	pattern := regexp.MustCompile(`^\$\((?P<env>\w+)\)$`)
	for i, arg := range function.Spec.Args {
		if pattern.MatchString(arg) {
			s.Log.V(0).Info("arg matches $()", "arg", arg)
			arg = pattern.ReplaceAllString(arg, "$env")
			function.Spec.Args[i] = vars[arg]
		}
	}
	s.Log.V(1).Info("about to run Function", "Function", name, "command", function.Spec.Command, "args", function.Spec.Args)
	if err := s.runTargetContainerChroot(function.Spec.Command[0],
		function.Spec.Args...); err != nil {
		s.Log.Error(err, "unable to run command", "command", function.Spec.Command)
		return err
	}
	return nil
}

// Runs the given command in the target container chroot
func (s Session) runTargetContainerChroot(runCmd string, args ...string) error {
	env := regexp.MustCompile(`/proc/[0-9]+/environ`)
	if err := filepath.WalkDir("/proc", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Skip process 1 and ourself
		if info.IsDir() && (info.Name() == "1" || info.Name() == strconv.Itoa(os.Getpid())) {
			return filepath.SkipDir
		}
		// Found an environ file. Start looking for TARGETCONTAINER_TAG
		if env.MatchString(path) {
			content, err := ioutil.ReadFile(path)
			// cannot read environ file. not the process we want to backup
			if err != nil {
				return fs.SkipDir
			}
			// Loops over the process environement variable looking for TARGETCONTAINER_TAG
			for _, env := range bytes.Split(content, []byte{'\000'}) {
				matched, err := regexp.Match(formolv1alpha1.TARGETCONTAINER_TAG, env)
				if err != nil {
					s.Log.Error(err, "unable to regexp", "env", string(env))
					return err
				}
				if matched {
					// Found the right process. Now run the command in its 'root'
					s.Log.V(0).Info("Found the tag", "file", path)
					root := filepath.Join(filepath.Dir(path), "root")
					if _, err := filepath.EvalSymlinks(root); err != nil {
						s.Log.Error(err, "cannot EvalSymlink.")
						return err
					}
					s.Log.V(0).Info("running cmd in chroot", "path", root)
					cmd := exec.Command("chroot", append([]string{root, runCmd}, args...)...)
					stdout, _ := cmd.StdoutPipe()
					stderr, _ := cmd.StderrPipe()
					_ = cmd.Start()

					scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
					scanner.Split(bufio.ScanLines)
					for scanner.Scan() {
						s.Log.V(0).Info("cmd output", "output", scanner.Text())
					}

					if err := cmd.Wait(); err != nil {
						return err
					} else {
						return filepath.SkipAll
					}
				}
			}
		}
		return nil
	}); err != nil {
		s.Log.Error(err, "cannot walk /proc")
		return err
	}
	return nil
}

func (s Session) runSteps(initializeSteps bool, target formolv1alpha1.Target) error {
	s.Log.V(0).Info("start to run the backup steps it any")
	// For every container listed in the target, run the initialization steps
	for _, container := range target.Containers {
		// Runs the steps one after the other
		for _, step := range container.Steps {
			if (initializeSteps == true && step.Finalize != nil && *step.Finalize == true) || (initializeSteps == false && (step.Finalize == nil || step.Finalize != nil && *step.Finalize == false)) {
				continue
			}
			return s.runFunction(step.Name)
		}
	}
	return nil
}

// Run the initializing steps in the INITIALIZING state of the controller
// before actualy doing the backup in the RUNNING state
func (s Session) runFinalizeSteps(target formolv1alpha1.Target) error {
	return s.runSteps(false, target)
}

// Run the finalizing steps in the FINALIZE state of the controller
// after the backup in the RUNNING state.
// The finalize happens whatever the result of the backup.
func (s Session) runInitializeSteps(target formolv1alpha1.Target) error {
	return s.runSteps(true, target)
}
