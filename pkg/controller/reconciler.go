package sentrycontroller

import (
	"context"
	"net/http"
	"reflect"
	"time"

	"github.com/pkg/errors"
	sentryv1alpha1 "github.com/sr/kube-sentry-controller/pkg/apis/sentry/v1alpha1"
	"github.com/sr/kube-sentry-controller/pkg/sentry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const finalizerName = "sentry.sr.github.com"

// reconcilerSet is a set of reconcile.Reconciler that reconcile Sentry API objects.
type reconcilerSet struct {
	scheme  *runtime.Scheme
	kube    client.Client // kubernetes API client
	sentry  sentry.Client // sentry API client
	org     string        // slug of the sentry organization being managed
	timeout time.Duration // timeout for reconcilation attempts
}

// +kubebuilder:rbac:groups=sentry.sr.github.com,resources=teams,verbs=get;list;watch;create;update;patch;delete
func (r *reconcilerSet) Team(request reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	instance := &sentryv1alpha1.Team{}
	err := r.kube.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	org, _, err := r.sentry.GetOrganization(ctx, r.org)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get organization %s", r.org)
	}

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !hasFinalizer(instance) {
			return reconcile.Result{}, err
		}

		if instance.Status.Slug != "" {
			resp, err := r.sentry.DeleteTeam(ctx, org.Slug, instance.Status.Slug)
			if err != nil && resp.StatusCode != 404 {
				return reconcile.Result{}, errors.Wrapf(err, "failed to delete team %s", instance.Status.Slug)
			}
		}

		instance.Status = sentryv1alpha1.TeamStatus{}
		removeFinalizer(instance)

		return reconcile.Result{}, r.kube.Update(ctx, instance)
	}

	if !hasFinalizer(instance) {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)

		if err := r.kube.Update(ctx, instance); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
		}
	}

	if instance.Status.Slug == "" {
		team, _, err := r.sentry.CreateTeam(ctx, org.Slug, instance.Spec.Name, "")
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to create team %s", instance.Spec.Name)
		}
		instance.Status.Slug = team.Slug

		return reconcile.Result{}, r.kube.Update(ctx, instance)
	}

	team, _, err := r.sentry.GetTeam(ctx, org.Slug, instance.Status.Slug)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get team %s", instance.Status.Slug)
	}

	if team.Name == instance.Spec.Name {
		return reconcile.Result{}, nil
	}

	_, err = r.sentry.UpdateTeamName(ctx, org.Slug, team.Slug, instance.Spec.Name)
	return reconcile.Result{}, err
}

// +kubebuilder:rbac:groups=sentry.sr.github.com,resources=sentryprojects,verbs=get;list;watch;create;update;patch;delete
func (r *reconcilerSet) Project(request reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	instance := &sentryv1alpha1.Project{}
	err := r.kube.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	org, _, err := r.sentry.GetOrganization(ctx, r.org)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get organization %s", r.org)
	}

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !hasFinalizer(instance) {
			return reconcile.Result{}, err
		}

		if instance.Status.Slug != "" {
			resp, err := r.sentry.DeleteProject(ctx, org.Slug, instance.Status.Slug)

			if err != nil && resp.StatusCode != http.StatusNotFound {
				return reconcile.Result{}, errors.Wrapf(err, "failed to delete project %s/%s", org.Slug, instance.Status.Slug)
			}
		}

		removeFinalizer(instance)
		instance.Status = sentryv1alpha1.ProjectStatus{}

		return reconcile.Result{}, r.kube.Update(ctx, instance)
	}

	if !hasFinalizer(instance) {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)

		if err := r.kube.Update(ctx, instance); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
		}
	}

	kubeTeam := &sentryv1alpha1.Team{}
	if err := r.kube.Get(
		ctx,
		client.ObjectKey{
			Namespace: instance.Spec.TeamRef.Namespace,
			Name:      instance.Spec.TeamRef.Name,
		},
		kubeTeam,
	); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to get team referenced by teamRef")
	}

	if instance.Status.Slug == "" {
		proj, _, err := r.sentry.CreateProject(ctx, org.Slug, kubeTeam.Status.Slug, instance.Spec.Name, "")
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to create project %s", instance.Spec.Name)
		}
		instance.Status.Slug = proj.Slug
		return reconcile.Result{}, r.kube.Update(ctx, instance)
	}

	proj, _, err := r.sentry.GetProject(ctx, org.Slug, instance.Status.Slug)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get project %s", instance.Status.Slug)
	}

	if proj.Name == instance.Spec.Name {
		return reconcile.Result{}, nil
	}

	if _, err := r.sentry.UpdateProjectName(ctx, org.Slug, proj.Slug, instance.Spec.Name); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to update project %s", instance.Status.Slug)
	}
	return reconcile.Result{}, nil
}

// +kubebuilder:rbac:groups=sentry.sr.github.com,resources=teams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
func (r *reconcilerSet) ClientKey(request reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	instance := &sentryv1alpha1.ClientKey{}
	err := r.kube.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	org, _, err := r.sentry.GetOrganization(ctx, r.org)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get organization %s", r.org)
	}

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !hasFinalizer(instance) {
			return reconcile.Result{}, nil
		}

		if instance.Status.ID != "" {
			resp, err := r.sentry.DeleteClientKey(ctx, org.Slug, instance.Status.Project, instance.Status.ID)

			if err != nil && resp.StatusCode != http.StatusNotFound {
				return reconcile.Result{}, errors.Wrapf(err, "failed to delete client key for project %s", instance.Status.Project)
			}
		}

		removeFinalizer(instance)
		instance.Status = sentryv1alpha1.ClientKeyStatus{}

		return reconcile.Result{}, r.kube.Update(ctx, instance)
	}

	if !hasFinalizer(instance) {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)

		if err := r.kube.Update(ctx, instance); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
		}
	}

	kubeProj := &sentryv1alpha1.Project{}
	if err := r.kube.Get(
		ctx,
		client.ObjectKey{
			Namespace: instance.Spec.ProjectRef.Namespace,
			Name:      instance.Spec.ProjectRef.Name,
		},
		kubeProj,
	); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to get project referenced in projectRef")
	}

	var key *sentry.ClientKey
	if instance.Status.ID == "" {
		key, _, err = r.sentry.CreateClientKey(ctx, org.Slug, kubeProj.Status.Slug, instance.Spec.Name)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to create client key for project %s", kubeProj.Status.Slug)
		}

		instance.Status.ID = key.ID
		instance.Status.Project = kubeProj.Status.Slug

		if err := r.kube.Update(ctx, instance); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		keys, _, err := r.sentry.GetClientKeys(ctx, org.Slug, kubeProj.Status.Slug)
		if err != nil {
			return reconcile.Result{}, err
		}
		for _, k := range keys {
			if k.ID == instance.Status.ID {
				key = k
				break
			}
		}
		if key == nil {
			return reconcile.Result{}, errors.New("key not found")
		}
	}

	if key.Name != instance.Spec.Name {
		if _, err := r.sentry.UpdateClientKeyName(ctx, org.Slug, kubeProj.Status.Slug, instance.Status.ID, instance.Spec.Name); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to rename client key")
		}
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: instance.Namespace,
			Name:      instance.Name,
		},
		Type: corev1.SecretType("Opaque"),
		Data: map[string][]byte{
			"dsn.secret": []byte(key.DSN.Secret),
			"dsn.csp":    []byte(key.DSN.CSP),
			"dsn.public": []byte(key.DSN.Public),
		},
	}

	if err := controllerutil.SetControllerReference(instance, secret, r.scheme); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to set controller reference on secret")
	}

	found := &corev1.Secret{}
	err = r.kube.Get(ctx, client.ObjectKey{Namespace: secret.Namespace, Name: secret.Name}, found)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}

		err := r.kube.Create(ctx, secret)
		return reconcile.Result{}, errors.Wrapf(err, "failed to create secret")
	}

	if reflect.DeepEqual(secret.Data, found.Data) {
		return reconcile.Result{}, nil
	}

	found.Data = secret.Data
	return reconcile.Result{}, r.kube.Update(ctx, found)
}

func hasFinalizer(obj metav1.Object) bool {
	for _, f := range obj.GetFinalizers() {
		if f == finalizerName {
			return true
		}
	}
	return false
}

func removeFinalizer(obj metav1.Object) {
	var finalizers []string
	for _, f := range obj.GetFinalizers() {
		if f != finalizerName {
			finalizers = append(finalizers, f)
		}
	}
	obj.SetFinalizers(finalizers)
}
