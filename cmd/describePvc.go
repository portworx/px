// Copyright © 2019 Portworx
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"text/tabwriter"

	"github.com/cheynewallace/tabby"
	"github.com/portworx/px/pkg/util"
	"github.com/spf13/cobra"
)

var describePvcCmd *cobra.Command

var _ = RegisterCommandVar(func() {
	describePvcCmd = &cobra.Command{
		Use:   "pvc",
		Short: "Describe Portworx volume for Kubernetes PVCs",
		Long:  "Show detailed information of Portworx volume for Kubernetes PVCs",
		Example: `1. Describe all pvcs that are Portworx volumes:
	$ px describe pvc
2. Describe specific pvc called pvc:
	$ px describe pvc abc
3. Describe list of pvcs (abc, xyz):
	$ px describe pvc abc xyz`,
		RunE: describePvcExec,
	}
})

// describePvcCmd represents the describePvc command
var _ = RegisterCommandInit(func() {
	describeCmd.AddCommand(describePvcCmd)
	describePvcCmd.Flags().StringP("namespace", "n", "", "Kubernetes namespace")
	describePvcCmd.Flags().Bool("all-namespaces", false, "Kubernetes namespace")
})

func describePvcExec(cmd *cobra.Command, args []string) error {
	// Parse out all of the common cli volume flags
	cvi := GetCliVolumeInputs(cmd, make([]string, 0))
	cvi.showK8s = true
	cvi.GetNamespace(cmd)

	// Create a cliVolumeOps object
	cvOps := NewCliVolumeOps(cvi)

	// Connect to px and k8s (if needed)
	err := cvOps.Connect()
	if err != nil {
		return err
	}
	defer cvOps.Close()

	// Create the parser object
	pdf := NewPvcDescribeFormatter(cvOps, args)

	// Print details and return any errors found during parsing
	return util.PrintFormatted(pdf)
}

type pvcDescribeFormatter struct {
	volumeDescribeFormatter
	pvcNames []string
}

func NewPvcDescribeFormatter(cvOps *cliVolumeOps, pvcNames []string) *pvcDescribeFormatter {
	vcf := NewVolumeDescribeFormatter(cvOps)
	return &pvcDescribeFormatter{
		volumeDescribeFormatter: *vcf,
		pvcNames:                pvcNames,
	}
}

// DefaultFormat returns the default string representation of the object
func (p *pvcDescribeFormatter) DefaultFormat() (string, error) {
	return p.toTabbed()
}

func (p *pvcDescribeFormatter) toTabbed() (string, error) {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	t := tabby.NewCustom(writer)

	allPvcs, err := p.pxVolumeOps.GetPxPvcs()
	if err != nil {
		return "", err
	}
	pvcs, err := filterPxPvcs(allPvcs, p.pvcNames)
	if err != nil {
		return "", err
	}

	if len(pvcs) == 0 {
		util.Printf("No resources found\n")
		return "", nil
	}

	for i, n := range pvcs {
		err = p.addVolumeDetails(n.GetVolume(), t, n.Pods)
		if err != nil {
			return "", err
		}
		// Put two empty lines between volumes
		if len(pvcs) > 1 && i != len(pvcs)-1 {
			t.AddLine("")
			t.AddLine("")
		}
	}
	t.Print()

	return b.String(), nil
}
