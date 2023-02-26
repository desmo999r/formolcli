package controllers

import (
	"bufio"
	"bytes"
	"encoding/json"
	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"io"
	"io/fs"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

var (
	REPOSITORY            string
	PASSWORD_FILE         string
	AWS_ACCESS_KEY_ID     string
	AWS_SECRET_ACCESS_KEY string
)

const (
	RESTIC_EXEC = "/usr/bin/restic"
)

func init() {
	REPOSITORY = os.Getenv(formolv1alpha1.RESTIC_REPOSITORY)
	PASSWORD_FILE = os.Getenv(formolv1alpha1.RESTIC_PASSWORD)
	AWS_ACCESS_KEY_ID = os.Getenv(formolv1alpha1.AWS_ACCESS_KEY_ID)
	AWS_SECRET_ACCESS_KEY = os.Getenv(formolv1alpha1.AWS_SECRET_ACCESS_KEY)
}

func (r *BackupSessionReconciler) getSecretData(name string) map[string][]byte {
	secret := corev1.Secret{}
	namespace := os.Getenv(formolv1alpha1.POD_NAMESPACE)
	if err := r.Get(r.Context, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &secret); err != nil {
		r.Log.Error(err, "unable to get Secret", "Secret", name)
		return nil
	}
	return secret.Data
}

func (r *BackupSessionReconciler) getEnvFromSecretKeyRef(name string, key string) string {
	if data := r.getSecretData(name); data != nil {
		return string(data[key])
	}
	return ""
}

func (r *BackupSessionReconciler) getConfigMapData(name string) map[string]string {
	configMap := corev1.ConfigMap{}
	namespace := os.Getenv(formolv1alpha1.POD_NAMESPACE)
	if err := r.Get(r.Context, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &configMap); err != nil {
		r.Log.Error(err, "unable to get ConfigMap", "configmap", name)
		return nil
	}
	return configMap.Data
}

func (r *BackupSessionReconciler) getEnvFromConfigMapKeyRef(name string, key string) string {
	if data := r.getConfigMapData(name); data != nil {
		return string(data[key])
	}
	return ""
}

func (r *BackupSessionReconciler) getFuncEnv(vars map[string]string, envVars []corev1.EnvVar) {
	for _, env := range envVars {
		if env.ValueFrom != nil {
			if env.ValueFrom.ConfigMapKeyRef != nil {
				vars[env.Name] = r.getEnvFromConfigMapKeyRef(env.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name, env.ValueFrom.ConfigMapKeyRef.Key)
			}
			if env.ValueFrom.SecretKeyRef != nil {
				vars[env.Name] = r.getEnvFromSecretKeyRef(env.ValueFrom.SecretKeyRef.LocalObjectReference.Name, env.ValueFrom.SecretKeyRef.Key)
			}
		} else {
			vars[env.Name] = env.Value
		}
	}
}

func (r *BackupSessionReconciler) getEnvFromSecretEnvSource(vars map[string]string, name string) {
	for key, value := range r.getSecretData(name) {
		vars[key] = string(value)
	}
}

func (r *BackupSessionReconciler) getEnvFromConfigMapEnvSource(vars map[string]string, name string) {
	for key, value := range r.getConfigMapData(name) {
		vars[key] = value
	}
}

func (r *BackupSessionReconciler) getFuncEnvFrom(vars map[string]string, envVars []corev1.EnvFromSource) {
	for _, env := range envVars {
		if env.ConfigMapRef != nil {
			r.getEnvFromConfigMapEnvSource(vars, env.ConfigMapRef.LocalObjectReference.Name)
		}
		if env.SecretRef != nil {
			r.getEnvFromSecretEnvSource(vars, env.SecretRef.LocalObjectReference.Name)
		}
	}
}

func (r *BackupSessionReconciler) getFuncVars(function formolv1alpha1.Function, vars map[string]string) {
	r.getFuncEnvFrom(vars, function.Spec.EnvFrom)
	r.getFuncEnv(vars, function.Spec.Env)
}

func (r *BackupSessionReconciler) runFunction(name string) error {
	namespace := os.Getenv(formolv1alpha1.POD_NAMESPACE)
	function := formolv1alpha1.Function{}
	if err := r.Get(r.Context, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &function); err != nil {
		r.Log.Error(err, "unable to get Function", "Function", name)
		return err
	}
	vars := make(map[string]string)
	r.getFuncVars(function, vars)

	r.Log.V(0).Info("function vars", "vars", vars)
	// Loop through the function.Spec.Command arguments to replace ${ARG}|$(ARG)|$ARG
	// with the environment variable value
	pattern := regexp.MustCompile(`^\$\((?P<env>\w+)\)$`)
	for i, arg := range function.Spec.Args {
		if pattern.MatchString(arg) {
			r.Log.V(0).Info("arg matches $()", "arg", arg)
			arg = pattern.ReplaceAllString(arg, "$env")
			function.Spec.Args[i] = vars[arg]
		}
	}
	r.Log.V(1).Info("about to run Function", "Function", name, "command", function.Spec.Command, "args", function.Spec.Args)
	if err := r.runTargetContainerChroot(function.Spec.Command[0],
		function.Spec.Args...); err != nil {
		r.Log.Error(err, "unable to run command", "command", function.Spec.Command)
		return err
	}
	return nil
}

func (r *BackupSessionReconciler) runBackupSteps(initializeSteps bool, target formolv1alpha1.Target) error {
	r.Log.V(0).Info("start to run the backup steps it any")
	// For every container listed in the target, run the initialization steps
	for _, container := range target.Containers {
		// Runs the steps one after the other
		for _, step := range container.Steps {
			if (initializeSteps == true && step.Finalize != nil && *step.Finalize == true) || (initializeSteps == false && (step.Finalize == nil || step.Finalize != nil && *step.Finalize == false)) {
				continue
			}
			return r.runFunction(step.Name)
		}
	}
	return nil
}

// Run the initializing steps in the INITIALIZING state of the controller
// before actualy doing the backup in the RUNNING state
func (r *BackupSessionReconciler) runFinalizeBackupSteps(target formolv1alpha1.Target) error {
	return r.runBackupSteps(false, target)
}

// Run the finalizing steps in the FINALIZE state of the controller
// after the backup in the RUNNING state.
// The finalize happens whatever the result of the backup.
func (r *BackupSessionReconciler) runInitializeBackupSteps(target formolv1alpha1.Target) error {
	return r.runBackupSteps(true, target)
}

// Runs the given command in the target container chroot
func (r *BackupSessionReconciler) runTargetContainerChroot(runCmd string, args ...string) error {
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
					r.Log.Error(err, "unable to regexp", "env", string(env))
					return err
				}
				if matched {
					// Found the right process. Now run the command in its 'root'
					r.Log.V(0).Info("Found the tag", "file", path)
					root := filepath.Join(filepath.Dir(path), "root")
					if _, err := filepath.EvalSymlinks(root); err != nil {
						r.Log.Error(err, "cannot EvalSymlink.")
						return err
					}
					r.Log.V(0).Info("running cmd in chroot", "path", root)
					cmd := exec.Command("chroot", append([]string{root, runCmd}, args...)...)
					stdout, _ := cmd.StdoutPipe()
					stderr, _ := cmd.StderrPipe()
					_ = cmd.Start()

					scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
					scanner.Split(bufio.ScanLines)
					for scanner.Scan() {
						r.Log.V(0).Info("cmd output", "output", scanner.Text())
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
		r.Log.Error(err, "cannot walk /proc")
		return err
	}
	return nil
}

func (r *BackupSessionReconciler) checkRepo(repo string) error {
	r.Log.V(0).Info("Checking repo", "repo", repo)
	if err := exec.Command(RESTIC_EXEC, "unlock", "-r", repo).Run(); err != nil {
		r.Log.Error(err, "unable to unlock repo", "repo", repo)
	}
	output, err := exec.Command(RESTIC_EXEC, "check", "-r", repo).CombinedOutput()
	if err != nil {
		r.Log.V(0).Info("Initializing new repo", "repo", repo)
		output, err = exec.Command(RESTIC_EXEC, "init", "-r", repo).CombinedOutput()
		if err != nil {
			r.Log.Error(err, "something went wrong during repo init", "output", output)
		}
	}
	return err
}

type BackupResult struct {
	SnapshotId string
	Duration   float64
}

func (r *BackupSessionReconciler) backupPaths(tag string, paths []string) (result BackupResult, err error) {
	if err = r.checkRepo(REPOSITORY); err != nil {
		r.Log.Error(err, "unable to setup repo", "repo", REPOSITORY)
		return
	}
	r.Log.V(0).Info("backing up paths", "paths", paths)
	cmd := exec.Command(RESTIC_EXEC, append([]string{"backup", "--json", "--tag", tag, "-r", REPOSITORY}, paths...)...)
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
