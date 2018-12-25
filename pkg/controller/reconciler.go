package controller

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
		found := false
		for _, f := range instance.ObjectMeta.Finalizers {
			if f == finalizerName {
				found = true
			}
		}
		if !found {
			return reconcile.Result{}, err
		}

		if instance.Status.Slug != "" {
			team, err := r.sentry.GetTeam(org, instance.Status.Slug)
			if err != nil {
				return reconcile.Result{}, errors.Wrapf(err, "failed to get team %s/%s", *org.Slug, instance.Status.Slug)
			}
			if err := r.sentry.DeleteTeam(org, team); err != nil {
				return reconcile.Result{}, errors.Wrapf(err, "failed to delete team %s/%s", *org.Slug, instance.Status.Slug)
			}
		}

		finalizers := []string{}
		for _, f := range instance.ObjectMeta.Finalizers {
			if f != finalizerName {
				finalizers = append(finalizers, f)
			}
		}
		instance.ObjectMeta.Finalizers = finalizers

		if err := r.kube.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{Requeue: true}, errors.Wrap(err, "failed to remove finalizer")
		}
		return reconcile.Result{}, nil
	}

	found := false
	for _, f := range instance.ObjectMeta.Finalizers {
		if f == finalizerName {
			found = true
		}
	}
	if !found {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)
	}
	if err := r.kube.Update(context.TODO(), instance); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
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
	team.Name = instance.Spec.Name
	if err := r.sentry.UpdateTeam(org, team); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to update team %s", instance.Status.Slug)
	}

	return reconcile.Result{}, nil
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
		found := false
		for _, f := range instance.ObjectMeta.Finalizers {
			if f == finalizerName {
				found = true
			}
		}
		if !found {
			return reconcile.Result{}, err
		}

		if instance.Status.Slug != "" {
			proj, err := r.sentry.GetProject(org, instance.Status.Slug)
			if err != nil {
				// Ignore 404 errors
				if v, ok := err.(sentry.APIError); !ok || v.StatusCode != 404 {
					return reconcile.Result{}, errors.Wrapf(err, "failed to get project %s/%s", *org.Slug, *proj.Slug)
				}
			}

			if err := r.sentry.DeleteProject(org, proj); err != nil {
				return reconcile.Result{}, errors.Wrapf(err, "failed to delete project %s/%s", *org.Slug, *proj.Slug)
			}
		}

		finalizers := []string{}
		for _, f := range instance.ObjectMeta.Finalizers {
			if f != finalizerName {
				finalizers = append(finalizers, f)
			}
		}
		instance.Status = sentryv1alpha1.ProjectStatus{}
		instance.ObjectMeta.Finalizers = finalizers

		if err := r.kube.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to remove finalizer")
		}
		return reconcile.Result{}, nil
	}

	var finalizer bool
	for _, f := range instance.ObjectMeta.Finalizers {
		if f == finalizerName {
			finalizer = true
		}
	}
	if !finalizer {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)

		if err := r.kube.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
		}
	}

	team, err := r.sentry.GetTeam(org, instance.Spec.Team)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get team %s/%s", *org.Slug, instance.Spec.Team)
	}

	if instance.Status.Slug == "" {
		proj, err := r.sentry.CreateProject(org, team, instance.Spec.Name, nil)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to create project %s", instance.Spec.Name)
		}
		instance.Status.Slug = *proj.Slug
		instance.Status.Team = *team.Slug
		return reconcile.Result{}, r.kube.Update(context.TODO(), instance)
	}

	proj, err := r.sentry.GetProject(org, instance.Status.Slug)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get project %s", instance.Status.Slug)
	}
	proj.Name = instance.Spec.Name
	if proj.Team == nil {
		proj.Team = &sentry.Team{}
	}
	proj.Team.Slug = &instance.Spec.Team
	if err := r.sentry.UpdateProject(org, proj); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to update project %s", instance.Status.Slug)
	}

	instance.Status.Team = *proj.Team.Slug
	if err := r.kube.Update(context.TODO(), instance); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
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
		found := false
		for _, f := range instance.ObjectMeta.Finalizers {
			if f == finalizerName {
				found = true
			}
		}
		if !found {
			return reconcile.Result{}, nil
		}

		if instance.Status.ID != "" {
			proj, err := r.sentry.GetProject(org, instance.Status.Project)
			if err != nil {
				return reconcile.Result{}, errors.Wrapf(err, "failed to get project %s/%s", *org.Slug, instance.Status.Project)
			}
			if err := r.sentry.DeleteClientKey(org, proj, sentry.Key{ID: instance.Status.ID}); err != nil {
				return reconcile.Result{}, errors.Wrapf(err, "failed to delete client key for project %s/%s", *org.Slug, *proj.Slug)
			}
		}

		finalizers := []string{}
		for _, f := range instance.ObjectMeta.Finalizers {
			if f != finalizerName {
				finalizers = append(finalizers, f)
			}
		}
		instance.ObjectMeta.Finalizers = finalizers
		instance.Status = sentryv1alpha1.ClientKeyStatus{}

		if err := r.kube.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to remove finalizer")
		}
		return reconcile.Result{}, nil
	}

	var fin bool
	for _, f := range instance.ObjectMeta.Finalizers {
		if f == finalizerName {
			fin = true
		}
	}
	if !fin {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerName)
	}
	if err := r.kube.Update(context.TODO(), instance); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
	}

	proj, err := r.sentry.GetProject(org, instance.Spec.Project)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get project %s", instance.Spec.Project)
	}

	var key sentry.Key
	if instance.Status.ID == "" {
		key, err = r.sentry.CreateClientKey(org, proj, instance.Spec.Name)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to create client key for project %s", instance.Spec.Project)
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
			return reconcile.Result{}, errors.Wrapf(err, "failed to get client keys for %s", instance.Spec.Project)
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

		if err := r.kube.Create(context.TODO(), secret); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to create secret")
		}
		return reconcile.Result{}, nil
	}
	if !reflect.DeepEqual(secret.Data, found.Data) {
		found.Data = secret.Data

		if err := r.kube.Update(context.TODO(), found); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to update secret %s/%s", secret.Namespace, secret.Name)
		}
	}

	return reconcile.Result{}, nil
}
