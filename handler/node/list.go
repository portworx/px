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
package node

import (
	"bytes"
	"fmt"
	"math/big"
	"text/tabwriter"

	"github.com/cheynewallace/tabby"
	humanize "github.com/dustin/go-humanize"
	api "github.com/libopenstorage/openstorage-sdk-clients/sdk/golang"
	"github.com/portworx/pxc/pkg/cliops"
	"github.com/portworx/pxc/pkg/commander"
	"github.com/portworx/pxc/pkg/portworx"
	"github.com/portworx/pxc/pkg/util"

	"github.com/spf13/cobra"
)

var getNodesCmd *cobra.Command

var _ = commander.RegisterCommandVar(func() {
	getNodesCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"get"},
		Short:   "List Portworx nodes information",
		RunE:    getNodesExec,
	}
})

var _ = commander.RegisterCommandInit(func() {
	NodeAddCommand(getNodesCmd)
	getNodesCmd.Flags().StringP("output", "o", "", "Output in yaml|json|wide")
	getNodesCmd.Flags().Bool("show-labels", false, "Show labels in the last column of the output")
})

func GetAddCommand(cmd *cobra.Command) {
	getNodesCmd.AddCommand(cmd)
}

func getNodesExec(cmd *cobra.Command, args []string) error {
	// Parse out all of the common cli volume flags
	cvi := cliops.NewCliInputs(cmd, args)

	// Create a cliOps object
	cliOps := cliops.NewCliOps(cvi)

	// Connect to pxc and k8s (if needed)
	err := cliOps.Connect()
	if err != nil {
		return err
	}
	defer cliOps.Close()

	// Create the parser object
	ngf := NewNodesGetFormatter(cliOps)

	// Print the details and return errors if any
	return util.PrintFormatted(ngf)

}

type nodesGetFormatter struct {
	util.BaseFormatOutput
	cliOps          cliops.CliOps
	nodeIdentifiers []string
	nodes           portworx.Nodes
}

func NewNodesGetFormatter(cliOps cliops.CliOps) *nodesGetFormatter {
	n := &nodesGetFormatter{
		cliOps:          cliOps,
		nodeIdentifiers: cliOps.CliInputs().Args,
		nodes:           portworx.NewNodes(cliOps.PxOps(), &portworx.NodeSpec{}),
	}
	n.FormatType = cliOps.CliInputs().FormatType
	return n
}

func (p *nodesGetFormatter) getNodes() ([]*api.StorageNode, error) {
	ns, err := p.nodes.GetNodes()
	if err != nil {
		return make([]*api.StorageNode, 0), err
	}

	nodes := make([]*api.StorageNode, 0, len(ns))

	// Store all of the found ids
	foundNodes := make(map[string]bool)
	for _, n := range ns {
		if len(p.nodeIdentifiers) != 0 {
			str, found := util.ListHaveMatch(p.nodeIdentifiers,
				[]string{n.GetId(),
					n.GetHostname(),
					n.GetMgmtIp(),
					n.GetSchedulerNodeName()})
			if found == false {
				continue
			} else {
				// Keep track of found nodes
				foundNodes[str] = true
			}
		}
		nodes = append(nodes, n)
	}

	// If some node is specified, and it is not found return error
	if len(p.nodeIdentifiers) != 0 {
		for _, f := range p.nodeIdentifiers {
			_, ok := foundNodes[f]
			if ok == false {
				return nodes, fmt.Errorf("Node with %s not found", f)
			}
		}
	}
	return nodes, nil
}

// YamlFormat returns the yaml representation of the object
func (p *nodesGetFormatter) YamlFormat() (string, error) {
	nodes, err := p.getNodes()
	if err != nil {
		return "", err
	}
	return util.ToYaml(nodes)
}

// JsonFormat returns the json representation of the object
func (p *nodesGetFormatter) JsonFormat() (string, error) {
	nodes, err := p.getNodes()
	if err != nil {
		return "", err
	}
	return util.ToJson(nodes)
}

// WideFormat returns the wide string representation of the object
func (p *nodesGetFormatter) WideFormat() (string, error) {
	return p.toTabbed()
}

// DefaultFormat returns the default string representation of the object
func (p *nodesGetFormatter) DefaultFormat() (string, error) {
	return p.toTabbed()
}

func (p *nodesGetFormatter) toTabbed() (string, error) {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	t := tabby.NewCustom(writer)

	nodes, err := p.getNodes()
	if err != nil {
		return "", util.PxErrorMessage(err, "Failed to get node list")
	}

	if len(nodes) == 0 {
		util.Printf("No resources found\n")
		return "", nil
	}

	// Start the columns
	t.AddHeader(p.getHeader()...)

	for _, n := range nodes {
		l, err := p.getLine(n)
		if err != nil {
			return "", nil
		}
		t.AddLine(l...)
	}
	t.Print()

	return b.String(), nil
}

func (p *nodesGetFormatter) getHeader() []interface{} {
	var header []interface{}
	if p.cliOps.CliInputs().Wide {
		header = []interface{}{"Id", "Hostname", "Version", "IP", "Data IP", "SchedulerNodeName", "Used", "Capacity", "# Disks", "# Pools", "Status"}
	} else {
		header = []interface{}{"Hostname", "Version", "SchedulerNodeName", "Used", "Capacity", "Status"}
	}
	if p.cliOps.CliInputs().ShowLabels {
		header = append(header, "Labels")
	}

	return header
}

func (p *nodesGetFormatter) getLine(n *api.StorageNode) ([]interface{}, error) {
	// Calculate used
	var (
		used, capacity uint64
	)
	for _, pool := range n.GetPools() {
		used += pool.GetUsed()
		capacity += pool.GetTotalSize()
	}

	usedStr := humanize.BigIBytes(big.NewInt(int64(used)))
	capacityStr := humanize.BigIBytes(big.NewInt(int64(capacity)))

	// Return a line
	var line []interface{}
	if p.cliOps.CliInputs().Wide {
		line = []interface{}{
			n.GetId(), n.GetHostname(), portworx.GetStorageNodeVersion(n), n.GetMgmtIp(),
			n.GetDataIp(), n.GetSchedulerNodeName(), usedStr, capacityStr,
			len(n.GetDisks()), len(n.GetPools()), util.SdkStatusToPrettyString(n.GetStatus()),
		}
	} else {
		line = []interface{}{
			n.GetHostname(), portworx.GetStorageNodeVersion(n),
			n.GetSchedulerNodeName(), usedStr, capacityStr,
			util.SdkStatusToPrettyString(n.GetStatus()),
		}
	}
	if p.cliOps.CliInputs().ShowLabels {
		line = append(line, util.StringMapToCommaString(n.GetNodeLabels()))
	}
	return line, nil
}
