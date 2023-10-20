/*
Copyright © 2017 Kubernetes Authors

Plugin source originally from:
github.com/kubernetes/kubernetes/pkg/kubectl/cmd/cmd.go

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/portworx/pxc/pkg/config"
	"github.com/portworx/pxc/pkg/portworx"
	"github.com/portworx/pxc/pkg/util"
)

// PluginHandler is capable of parsing command line arguments
// and performing executable filename lookups to search
// for valid plugin files, and execute found plugins.
type PluginHandler interface {
	// exists at the given filename, or a boolean false.
	// Lookup will iterate over a list of given prefixes
	// in order to recognize valid plugin filenames.
	// The first filepath to match a prefix is returned.
	Lookup(filename string) (string, bool)
	// Execute receives an executable's filepath, a slice
	// of arguments, and a slice of environment variables
	// to relay to the executable.
	Execute(executablePath string, cmdArgs, environment []string) error
}

// DefaultPluginHandler implements PluginHandler
type DefaultPluginHandler struct {
	ValidPrefixes []string
}

// NewDefaultPluginHandler instantiates the DefaultPluginHandler with a list of
// given filename prefixes used to identify valid plugin filenames.
func NewDefaultPluginHandler(validPrefixes []string) *DefaultPluginHandler {
	return &DefaultPluginHandler{
		ValidPrefixes: validPrefixes,
	}
}

// Lookup implements PluginHandler
func (h *DefaultPluginHandler) Lookup(filename string) (string, bool) {
	for _, prefix := range h.ValidPrefixes {
		name := fmt.Sprintf("%s-%s", prefix, filename)
		// place the config dir bin in the path
		path, err := exec.LookPath(name)
		if err != nil || len(path) == 0 {
			// Look in (configFlags.ConfigDir)/bin
			path, err = exec.LookPath(filepath.Join(config.CM().GetFlags().ConfigDir, "bin", name))
			if err != nil || len(path) == 0 {
				continue
			}
		}
		return path, true
	}

	return "", false
}

// Execute implements PluginHandler
func (h *DefaultPluginHandler) Execute(executablePath string, cmdArgs, environment []string) error {

	// Windows does not support exec syscall.
	if runtime.GOOS == "windows" {
		cmd := exec.Command(executablePath, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = environment
		err := cmd.Run()
		if err == nil {
			os.Exit(0)
		}
		return err
	}

	// invoke cmd binary relaying the environment and args given
	// append executablePath to cmdArgs, as execve will make first argument the "binary name".
	return syscall.Exec(executablePath, append([]string{executablePath}, cmdArgs...), environment)
}

// HandlePluginCommand receives a pluginHandler and command-line arguments and attempts to find
// a plugin executable on the PATH that satisfies the given arguments.
func HandlePluginCommand(pluginHandler PluginHandler, cmdArgs []string) error {
	remainingArgs := []string{} // all "non-flag" arguments

	for idx := range cmdArgs {
		if strings.HasPrefix(cmdArgs[idx], "-") {
			break
		}
		remainingArgs = append(remainingArgs, strings.Replace(cmdArgs[idx], "-", "_", -1))
	}

	foundBinaryPath := ""

	// attempt to find binary, starting at longest possible name with given cmdArgs
	for len(remainingArgs) > 0 {
		path, found := pluginHandler.Lookup(strings.Join(remainingArgs, "-"))
		if !found {
			remainingArgs = remainingArgs[:len(remainingArgs)-1]
			continue
		}

		foundBinaryPath = path
		break
	}

	if len(foundBinaryPath) == 0 {
		return nil
	}

	// Get token
	var err error
	authInfo := config.CM().GetCurrentAuthInfo()
	token := authInfo.Token
	if len(authInfo.KubernetesAuthInfo.SecretName) != 0 &&
		len(authInfo.KubernetesAuthInfo.SecretNamespace) != 0 {
		token, err = portworx.PxGetTokenFromSecret(authInfo.KubernetesAuthInfo.SecretName, authInfo.KubernetesAuthInfo.SecretNamespace)
		if err != nil {
			return err
		}
	}

	// Setup environment variables
	envVars := appendToEnv(os.Environ(), util.EvInKubectlPluginMode, util.InKubectlPluginMode())
	envVars = appendToEnv(envVars, util.EvPxcToken, token)

	currentCluster := config.CM().GetCurrentCluster()
	envVars = appendToEnv(envVars, util.EvPortworxServiceName, currentCluster.TunnelServiceName)
	envVars = appendToEnv(envVars, util.EvPortworxServiceNamespace, currentCluster.TunnelServiceNamespace)
	envVars = appendToEnv(envVars, util.EvPortworxServicePort, currentCluster.TunnelServicePort)

	// invoke cmd binary relaying the current environment and args given
	if err := pluginHandler.Execute(foundBinaryPath, cmdArgs[len(remainingArgs):], envVars); err != nil {
		return err
	}

	return nil
}

func appendToEnv(env []string, key string, val interface{}) []string {
	return append(env, fmt.Sprintf("%s=%v", key, val))
}
