/*
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

package bootstrap

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

type OKE struct {
	Options
	ContainerRuntime string
}

func (e OKE) Script() (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(e.okeBootstrapScript())), nil
}

//nolint:gocyclo
func (e OKE) okeBootstrapScript() string {
	var caBundleArg string
	if e.CABundle != nil {
		caBundleArg = fmt.Sprintf("--kubelet-ca-cert '%s'", *e.CABundle)
	}
	var userData bytes.Buffer
	userData.WriteString("#!/bin/bash\n")
	// Due to the way bootstrap.sh is written, parameters should not be passed to it with an equal sign
	url, _ := url.Parse(e.ClusterEndpoint)
	userData.WriteString(fmt.Sprintf("bash /etc/oke/oke-install.sh --apiserver-endpoint '%s' %s", url.Hostname(), caBundleArg))
	if args := e.kubeletExtraArgs(); len(args) > 0 {
		userData.WriteString(fmt.Sprintf(" \\\n--kubelet-extra-args '%s'", strings.Join(args, " ")))
	}

	if e.KubeletConfig != nil && len(e.KubeletConfig.ClusterDNS) > 0 {
		userData.WriteString(fmt.Sprintf(" \\\n--cluster-dns '%s'", e.KubeletConfig.ClusterDNS[0]))
	} else if e.ClusterDns != "" {
		userData.WriteString(fmt.Sprintf(" \\\n--cluster-dns '%s'", e.ClusterDns))
	}

	return userData.String()
}

// kubeletExtraArgs for the EKS bootstrap.sh script uses the concept of ENI-limited pod density to set pods
// If this argument is explicitly disabled, then set the max-pods value on the kubelet to the static value of 110
func (e OKE) kubeletExtraArgs() []string {
	args := e.Options.kubeletExtraArgs()
	return args
}
