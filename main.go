package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	sentry "github.com/atlassian/go-sentry-api"
	"github.com/pkg/errors"
	"github.com/sr/kube-sentry-controller/pkg/apis"
	"github.com/sr/kube-sentry-controller/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "kube-sentry-controller: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	opts := &struct {
		org         string
		apiEndpoint string
		apiToken    string
		apiTimeout  time.Duration
	}{
		apiEndpoint: "https://sentry.io/api/0/",
		apiTimeout:  5 * time.Second,
	}

	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&opts.org, "organization", opts.org, "Slug of the Sentry organization")
	fs.StringVar(&opts.apiEndpoint, "api-endpoint", opts.apiEndpoint, "Sentry API endpoint")
	fs.StringVar(&opts.apiToken, "api-token", "", "Sentry API auth token")
	fs.DurationVar(&opts.apiTimeout, "api-timeout", opts.apiTimeout, "Sentry API request timeout")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if opts.org == "" {
		return fmt.Errorf("required flag missing: organization")
	}
	if opts.apiToken == "" {
		return fmt.Errorf("required flag missing: api-token")
	}

	logf.SetLogger(logf.ZapLogger(true))
	logger := logf.Log.WithName("kube-sentry-controller")

	cfg, err := config.GetConfig()
	if err != nil {
		return errors.Wrap(err, "failed to set up kubernetes client config")
	}

	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		return errors.Wrap(err, "failed to set up controller manager")
	}

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		return errors.Wrap(err, "failed to add APIs to scheme")
	}

	sentry, err := sentry.NewClient(opts.apiToken, &opts.apiEndpoint, nil)
	if err != nil {
		return errors.Wrap(err, "failed to set up sentry client")
	}

	if err := controller.New(mgr, logger, sentry, opts.org); err != nil {
		return errors.Wrap(err, "failed to registry sentry controllers with the manager")
	}

	logger.Info("starting...")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		return errors.Wrap(err, "failed to run the manager")
	}
	logger.Info("exiting...")
	return nil
}
