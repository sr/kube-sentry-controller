// +build tools

package tools

import (
	_ "k8s.io/code-generator/cmd/client-gen" # for go generate
	_ "k8s.io/code-generator/cmd/deepcopy-gen" # for go generate
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
