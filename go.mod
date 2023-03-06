module github.com/sr/kube-sentry-controller

go 1.12

require (
	github.com/go-logr/logr v0.2.0
	github.com/gobuffalo/flect v0.1.6 // indirect
	github.com/golangci/golangci-lint v1.21.0
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.1.0 // indirect
	github.com/prometheus/procfs v0.0.4 // indirect
	k8s.io/api v0.20.0-alpha.2
	k8s.io/apimachinery v0.20.0-alpha.2
	k8s.io/client-go v0.20.0-alpha.2
	k8s.io/code-generator v0.0.0-20190831074504-732c9ca86353
	sigs.k8s.io/controller-runtime v0.3.0
	sigs.k8s.io/controller-tools v0.2.0
)
