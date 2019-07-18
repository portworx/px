/*
Copyright © 2019 Portworx

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
package kubernetes

import (
	"fmt"
	"os"

	"github.com/portworx/px/pkg/contextconfig"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// KubeConnect will return a Kubernetes client using the kubeconfig file
// set in the default context.
// clientcmd.ClientConfig will allow the caller to call ClientConfig.Namespace() to get the namespace
// set by the caller on their Kubeconfig.
func KubeConnect(cfgFile, context string) (clientcmd.ClientConfig, *kubernetes.Clientset, error) {
	var (
		kubeconfig string
		pxctx      *contextconfig.ClientContext
		err        error
	)

	if len(context) == 0 {
		pxctx, err = contextconfig.NewConfigReference(cfgFile).GetCurrent()
	} else {
		pxctx, err = contextconfig.NewConfigReference(cfgFile).GetNamedContext(context)
	}
	if err != nil {
		return nil, nil, err
	}
	if len(pxctx.Kubeconfig) == 0 {
		kubeconfig = os.Getenv("KUBECONFIG")
	} else {
		kubeconfig = pxctx.Kubeconfig
	}
	if len(kubeconfig) == 0 {
		return nil, nil, fmt.Errorf("No kubeconfig found in context %s\n", pxctx.Name)
	}

	// Get the client config
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}})
	r, err := cc.ClientConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to configure kubernetes client: %v\n", err)
	}
	// Get a client to the Kuberntes server
	clientset, err := kubernetes.NewForConfig(r)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to connect to Kubernetes: %v\n", err)
	}

	return cc, clientset, nil
}
