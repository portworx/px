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
package cmd

import (
	"net"
	"strings"

	api "github.com/libopenstorage/openstorage-sdk-clients/sdk/golang"
	"github.com/portworx/px/pkg/contextconfig"
	"github.com/portworx/px/pkg/portworx"
	"github.com/portworx/px/pkg/util"
	"github.com/spf13/cobra"
)

type createClusterpairOpts struct {
	req             *api.ClusterPairCreateRequest
	source          string
	destination     string
	destinationPort uint32
	mode            string
}

var (
	ccpOpts = createClusterpairOpts{
		req: &api.ClusterPairCreateRequest{},
	}
)

// createClusterpairCmd represents the createClusterpair command
var createClusterpairCmd = &cobra.Command{
	Use:     "clusterpair",
	Aliases: []string{"clusterpairs"},
	Short:   "Pair this cluster with another Portworx cluster",
	Long: `TODO

ADD EXAMPLES
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return createClusterpairExec(cmd, args)
	},
}

func init() {
	createCmd.AddCommand(createClusterpairCmd)

	createClusterpairCmd.Flags().StringVarP(&ccpOpts.source, "source", "s", "", "Context for the source cluster (required)")
	createClusterpairCmd.Flags().StringVarP(&ccpOpts.destination, "destination", "d", "", "Context for the destination cluster (required)")
	createClusterpairCmd.Flags().Uint32VarP(&ccpOpts.destinationPort, "destination-port", "p", 9001,
		"Port for destination cluster (optional)")
	createClusterpairCmd.Flags().StringVarP(&ccpOpts.mode, "mode", "m", "", "Pairing mode to use (optional)")
	createClusterpairCmd.Flags().BoolVarP(&ccpOpts.req.SetDefault, "set-default", "", false, "Set this as the default cluster pair (optional)")
	createClusterpairCmd.Flags().SortFlags = false
	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createClusterpairCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// createClusterpairCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func createClusterpairExec(cmd *cobra.Command, args []string) error {
	// Get connection info for destination cluster and remote cluster pair request
	destContext, err := contextconfig.NewConfigReference(GetConfigFile()).GetNamedContext(ccpOpts.destination)
	if err != nil {
		return util.PxErrorMessage(err, "Failed to get destination context")
	}
	destHost, _, err := net.SplitHostPort(destContext.Endpoint)
	if err != nil {
		return util.PxErrorMessage(err, "Failed to get cluster token")
	}
	ccpOpts.req.RemoteClusterIp = destHost
	ccpOpts.req.RemoteClusterPort = ccpOpts.destinationPort

	// Add mode to request
	ccpOpts.mode = strings.ToLower(ccpOpts.mode)
	switch {
	case ccpOpts.mode == "dr" || ccpOpts.mode == "disasterrecovery":
		ccpOpts.req.Mode = api.ClusterPairMode_DisasterRecovery
	default:
		ccpOpts.req.Mode = api.ClusterPairMode_Default
	}

	// Connect to source
	ctxSource, connSource, err := portworx.PxConnectNamed(GetConfigFile(), ccpOpts.source)
	if err != nil {
		return util.PxErrorMessagef(err, "Failed to connect to %s", ccpOpts.source)
	}
	defer connSource.Close()
	clusterpairsSource := api.NewOpenStorageClusterPairClient(connSource)

	// Connect to destination
	ctxDest, connDest, err := portworx.PxConnectNamed(GetConfigFile(), ccpOpts.destination)
	if err != nil {
		return util.PxErrorMessagef(err, "Failed to connect to %s", ccpOpts.destination)
	}
	defer connDest.Close()
	clusterpairsDest := api.NewOpenStorageClusterPairClient(connDest)

	// Get token from destination cluster
	tokenResp, err := clusterpairsDest.GetToken(ctxDest, &api.SdkClusterPairGetTokenRequest{})
	if err != nil {
		return util.PxErrorMessage(err, "Failed to get cluster token")
	}
	ccpOpts.req.RemoteClusterToken = tokenResp.Result.Token

	// Create pair from source to destination cluster
	_, err = clusterpairsSource.Create(ctxSource, &api.SdkClusterPairCreateRequest{
		Request: ccpOpts.req,
	})
	if err != nil {
		return util.PxErrorMessage(err, "Failed to create cluster pair")
	}

	// Show user information
	util.Printf("Cluster pair created from %s to %s\n",
		ccpOpts.source,
		ccpOpts.destination,
	)
	return nil
}
