package sentrycontroller

import (
	"github.com/go-logr/logr"
	sentryv1alpha1 "github.com/sr/kube-sentry-controller/pkg/apis/sentry/v1alpha1"
	"github.com/sr/kube-sentry-controller/pkg/sentry"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// New initializes the Sentry controller and adds it to controller runtime manager.
func New(mgr manager.Manager, logger logr.Logger, sentry sentry.Client, org string) error {
	r := &reconcilerSet{
		scheme: mgr.GetScheme(),
		kube:   mgr.GetClient(),
		sentry: sentry,
		org:    org,
	}

	c, err := controller.New("sentry-team", mgr, controller.Options{
		Reconciler: reconcile.Func(r.Team),
	})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &sentryv1alpha1.Team{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	c, err = controller.New("sentry-project", mgr, controller.Options{
		Reconciler: reconcile.Func(r.Project),
	})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &sentryv1alpha1.Project{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	c, err = controller.New("sentry-clientkey", mgr, controller.Options{
		Reconciler: reconcile.Func(r.ClientKey),
	})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &sentryv1alpha1.ClientKey{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &sentryv1alpha1.ClientKey{},
		},
	)
}
