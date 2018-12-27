package sentrycontroller

import (
	"context"
	"fmt"
	"reflect"

	sentry "github.com/atlassian/go-sentry-api"
	"github.com/pkg/errors"
	sentryv1alpha1 "github.com/sr/kube-sentry-controller/pkg/apis/sentry/v1alpha1"
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
	scheme *runtime.Scheme
	kube   client.Client // kubernetes API client
	sentry SentryClient  // sentry API client
	org    string        // slug of the sentry organization being managed
}

// +kubebuilder:rbac:groups=sentry.sr.github.com,resources=teams,verbs=get;list;watch;create;update;patch;delete
func (r *reconcilerSet) Team(request reconcile.Request) (reconcile.Result, error) {
	instance := &sentryv1alpha1.Team{}
	err := r.kube.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	org, err := r.sentry.GetOrganization(r.org)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get organization %s", r.org)
	}

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !hasFinalizer(instance) {
			return reconcile.Result{}, err
		}

		if instance.Status.Slug != "" {
			err := r.sentry.DeleteTeam(org, sentry.Team{Slug: &instance.Status.Slug})

			if err != nil && !isNotFound(err) {
				return reconcile.Result{}, errors.Wrapf(err, "failed to delete team %s", instance.Status.Slug)
			}
		}

		instance.Status = sentryv1alpha1.TeamStatus{}
		removeFinalizer(instance)

		return reconcile.Result{}, r.kube.Update(context.TODO(), instance)
	}

	if !hasFinalizer(instance) {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)

		if err := r.kube.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
		}
	}

	if instance.Status.Slug == "" {
		team, err := r.sentry.CreateTeam(org, instance.Spec.Name, nil)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to create team %s", instance.Spec.Name)
		}
		instance.Status.Slug = *team.Slug

		return reconcile.Result{}, r.kube.Update(context.TODO(), instance)
	}

	team, err := r.sentry.GetTeam(org, instance.Status.Slug)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get team %s", instance.Status.Slug)
	}

	if team.Name == instance.Spec.Name {
		return reconcile.Result{}, nil
	}

	team.Name = instance.Spec.Name
	return reconcile.Result{}, r.sentry.UpdateTeam(org, team)
}

// +kubebuilder:rbac:groups=sentry.sr.github.com,resources=sentryprojects,verbs=get;list;watch;create;update;patch;delete
func (r *reconcilerSet) Project(request reconcile.Request) (reconcile.Result, error) {
	instance := &sentryv1alpha1.Project{}
	err := r.kube.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	org, err := r.sentry.GetOrganization(r.org)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get organization %s", r.org)
	}

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !hasFinalizer(instance) {
			return reconcile.Result{}, err
		}

		if instance.Status.Slug != "" {
			err := r.sentry.DeleteProject(org, sentry.Project{Slug: &instance.Status.Slug})

			if err != nil && !isNotFound(err) {
				return reconcile.Result{}, errors.Wrapf(err, "failed to delete project %s/%s", *org.Slug, instance.Status.Slug)
			}
		}

		removeFinalizer(instance)
		instance.Status = sentryv1alpha1.ProjectStatus{}

		return reconcile.Result{}, r.kube.Update(context.TODO(), instance)
	}

	if !hasFinalizer(instance) {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)

		if err := r.kube.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
		}
	}

	kubeTeam := &sentryv1alpha1.Team{}
	if err := r.kube.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: instance.Spec.TeamRef.Namespace,
			Name:      instance.Spec.TeamRef.Name,
		},
		kubeTeam,
	); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to get team referenced by teamRef")
	}

	if instance.Status.Slug == "" {
		proj, err := r.sentry.CreateProject(org, sentry.Team{Slug: &instance.Status.Slug}, instance.Spec.Name, nil)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to create project %s", instance.Spec.Name)
		}
		instance.Status.Slug = *proj.Slug
		return reconcile.Result{}, r.kube.Update(context.TODO(), instance)
	}

	proj, err := r.sentry.GetProject(org, instance.Status.Slug)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get project %s", instance.Status.Slug)
	}

	if proj.Name == instance.Spec.Name {
		return reconcile.Result{}, nil
	}

	// TODO(sr) Updating the team is no longer supported by the Sentry API.
	// See https://github.com/getsentry/sentry/blob/master/src/sentry/api/endpoints/project_details.py#L296-L302
	proj.Team = nil
	proj.Name = instance.Spec.Name

	err = r.sentry.UpdateProject(org, proj)
	return reconcile.Result{}, errors.Wrapf(err, "failed to update project %s", instance.Status.Slug)
}

// +kubebuilder:rbac:groups=sentry.sr.github.com,resources=teams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
func (r *reconcilerSet) ClientKey(request reconcile.Request) (reconcile.Result, error) {
	instance := &sentryv1alpha1.ClientKey{}
	err := r.kube.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	org, err := r.sentry.GetOrganization(r.org)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get organization %s", r.org)
	}

	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !hasFinalizer(instance) {
			return reconcile.Result{}, nil
		}

		if instance.Status.ID != "" {
			err := r.sentry.DeleteClientKey(org, sentry.Project{Slug: &instance.Status.Project}, sentry.Key{ID: instance.Status.ID})
			if err != nil && !isNotFound(err) {
				return reconcile.Result{}, errors.Wrapf(err, "failed to delete client key for project %s", instance.Status.Project)
			}
		}

		removeFinalizer(instance)
		instance.Status = sentryv1alpha1.ClientKeyStatus{}

		return reconcile.Result{}, r.kube.Update(context.TODO(), instance)
	}

	if !hasFinalizer(instance) {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)

		if err := r.kube.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
		}
	}

	kubeProj := &sentryv1alpha1.Project{}
	k := client.ObjectKey{Namespace: instance.Spec.ProjectRef.Namespace, Name: instance.Spec.ProjectRef.Name}
	if err := r.kube.Get(context.TODO(), k, kubeProj); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to get project referenced in projectRef")
	}

	proj, err := r.sentry.GetProject(org, kubeProj.Status.Slug)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get project %s", kubeProj.Status.Slug)
	}

	var key sentry.Key
	if instance.Status.ID == "" {
		key, err = r.sentry.CreateClientKey(org, proj, instance.Spec.Name)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to create client key for project %s", *proj.Slug)
		}
		instance.Status.ID = key.ID
		instance.Status.Project = *proj.Slug
		if err := r.kube.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	if key.ID == "" {
		keys, err := r.sentry.GetClientKeys(org, proj)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to get client keys for %s", *proj.Slug)
		}
		for _, k := range keys {
			if k.ID == instance.Status.ID {
				key = k
				break
			}
		}
		if key.ID == "" {
			return reconcile.Result{}, fmt.Errorf("key id %s not found for project %s", instance.Status.ID, *proj.Slug)
		}
	}

	if key.Label != instance.Spec.Name {
		if _, err := r.sentry.UpdateClientKey(org, proj, key, instance.Spec.Name); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to update client key id %s for project %s", key.ID, *proj.Slug)
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
	err = r.kube.Get(context.TODO(), client.ObjectKey{Namespace: secret.Namespace, Name: secret.Name}, found)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}

		err := r.kube.Create(context.TODO(), secret)
		return reconcile.Result{}, errors.Wrapf(err, "failed to create secret")
	}

	if reflect.DeepEqual(secret.Data, found.Data) {
		return reconcile.Result{}, nil
	}

	found.Data = secret.Data
	return reconcile.Result{}, r.kube.Update(context.TODO(), found)
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

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	v, ok := err.(sentry.APIError)
	if !ok {
		return false
	}
	return (v.StatusCode == 404)
}
