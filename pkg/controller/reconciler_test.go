package sentrycontroller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	sentryv1alpha1 "github.com/sr/kube-sentry-controller/pkg/apis/sentry/v1alpha1"
	sentry "github.com/sr/kube-sentry-controller/pkg/sentry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	scheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestClientKeyReconciler(t *testing.T) {
	if err := sentryv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatal(err)
	}

	testClientKey := &sentryv1alpha1.ClientKey{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "testing",
			Name:      "test-key",
		},
		Spec: sentryv1alpha1.ClientKeySpec{
			Name:        "My Key",
			ProjectSlug: "test-proj",
		},
	}

	for _, tc := range []struct {
		name   string
		kube   []runtime.Object
		sentry *sentry.Fake
		req    reconcile.Request

		wantErr           error
		wantClientKeys    []*sentry.ClientKey
		wantKubeClientKey *sentryv1alpha1.ClientKey
		wantKubeSecrets   []*corev1.Secret
	}{
		{
			name: "object is not found",
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "not-found", Name: "not-found"},
			},
			sentry:  &sentry.Fake{},
			wantErr: nil,
		},
		{
			name: "errors if organization does not exist",
			kube: []runtime.Object{testClientKey},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test-key"},
			},
			sentry:  &sentry.Fake{},
			wantErr: errors.New("organization not found"),
		},
		{
			name: "errors if project does not exist",
			kube: []runtime.Object{
				&sentryv1alpha1.ClientKey{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "testing",
					},
					Spec: sentryv1alpha1.ClientKeySpec{
						Name:             "My Test Project",
						ProjectSlug:      "not-found",
						OrganizationSlug: "my-sentry-org",
					},
				},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "my-sentry-org",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			wantErr: errors.New("failed to create client key"),
			wantKubeClientKey: &sentryv1alpha1.ClientKey{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.ClientKeyStatus{
					ID: "",
				},
			},
		},
		{
			name: "errors if project does not exist",
			kube: []runtime.Object{
				&sentryv1alpha1.ClientKey{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "test-key",
					},
					Spec: sentryv1alpha1.ClientKeySpec{
						Name:             "My Key",
						ProjectSlug:      "test-proj",
						OrganizationSlug: "my-sentry-org",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test-key"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "my-sentry-org",
					},
				},
			},
			wantErr: errors.New("failed to create client key for project"),
		},
		{
			name: "creates sentry client key and secret",
			kube: []runtime.Object{
				&sentryv1alpha1.ClientKey{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "sentry-key-1",
					},
					Spec: sentryv1alpha1.ClientKeySpec{
						Name:             "My Key",
						ProjectSlug:      "test-proj",
						OrganizationSlug: "my-sentry-org",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "sentry-key-1"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "my-sentry-org",
					},
				},
				Projects: []*sentry.Project{
					{
						Slug: "test-proj",
					},
				},
			},
			wantClientKeys: []*sentry.ClientKey{
				{
					ID:   "1",
					Name: "My Key",
				},
			},
			wantKubeClientKey: &sentryv1alpha1.ClientKey{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.ClientKeyStatus{
					ID:               "1",
					ProjectSlug:      "test-proj",
					OrganizationSlug: "my-sentry-org",
				},
			},
			wantKubeSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "sentry-key-1",
					},
					Data: map[string][]byte{
						"dsn.public": []byte("public"),
						"dsn.secret": []byte("secret"),
						"dsn.csp":    []byte("csp"),
					},
				},
			},
		},
		{
			name: "updates sentry client key and corresponding secret",
			kube: []runtime.Object{
				&sentryv1alpha1.ClientKey{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "test-key",
					},
					Spec: sentryv1alpha1.ClientKeySpec{
						Name:             "new key name",
						ProjectSlug:      "test-proj",
						OrganizationSlug: "my-sentry-org",
					},
					Status: sentryv1alpha1.ClientKeyStatus{
						ID:               "1",
						ProjectSlug:      "test-proj",
						OrganizationSlug: "my-sentry-org",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "test-key",
					},
					Data: map[string][]byte{
						"dsn.public": []byte("public"),
						"dsn.secret": []byte("secret"),
						"dsn.csp":    []byte("csp"),
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test-key"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "my-sentry-org",
					},
				},
				Projects: []*sentry.Project{
					{
						Slug: "test-proj",
					},
				},
				ClientKeys: []*sentry.ClientKey{
					{
						ID:   "1",
						Name: "old key name",
						DSN: &sentry.ClientKeyDSN{
							Public: "new public",
							CSP:    "new csp",
							Secret: "new secret",
						},
					},
				},
			},
			wantClientKeys: []*sentry.ClientKey{
				{
					ID:   "1",
					Name: "new key name",
				},
			},
			wantKubeClientKey: &sentryv1alpha1.ClientKey{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.ClientKeyStatus{
					ID:               "1",
					ProjectSlug:      "test-proj",
					OrganizationSlug: "my-sentry-org",
				},
			},
			wantKubeSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "test-key",
					},
					Data: map[string][]byte{
						"dsn.public": []byte("new public"),
						"dsn.secret": []byte("new secret"),
						"dsn.csp":    []byte("new csp"),
					},
				},
			},
		},
		{
			name: "deletes sentry client key",
			kube: []runtime.Object{
				&sentryv1alpha1.ClientKey{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "testing",
						Name:              "test-key",
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
						Finalizers:        []string{finalizerName},
					},
					Spec: sentryv1alpha1.ClientKeySpec{
						Name:             "new key name",
						ProjectSlug:      "test-proj",
						OrganizationSlug: "my-sentry-org",
					},
					Status: sentryv1alpha1.ClientKeyStatus{
						ID:               "1",
						ProjectSlug:      "test-proj",
						OrganizationSlug: "my-sentry-org",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test-key"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "my-sentry-org",
					},
				},
				Projects: []*sentry.Project{
					{
						Slug: "test-proj",
					},
				},
				ClientKeys: []*sentry.ClientKey{
					{
						ID:   "1",
						Name: "key name",
						DSN: &sentry.ClientKeyDSN{
							Public: "public",
							CSP:    "csp",
							Secret: "secret",
						},
					},
					{
						ID:   "2",
						Name: "some other key",
						DSN: &sentry.ClientKeyDSN{
							Public: "public",
							CSP:    "csp",
							Secret: "secret",
						},
					},
				},
			},
			wantClientKeys: []*sentry.ClientKey{
				{
					ID:   "2",
					Name: "some other key",
				},
			},
			wantKubeClientKey: &sentryv1alpha1.ClientKey{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: nil,
				},
			},
		},
		{
			name: "delete noops when project has already been deleted",
			kube: []runtime.Object{
				&sentryv1alpha1.ClientKey{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "testing",
						Name:              "test-key",
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
						Finalizers:        []string{finalizerName},
					},
					Spec: sentryv1alpha1.ClientKeySpec{
						Name:             "new key name",
						ProjectSlug:      "test-proj",
						OrganizationSlug: "my-sentry-org",
					},
					Status: sentryv1alpha1.ClientKeyStatus{
						ID:               "1",
						OrganizationSlug: "my-sentry-org",
						ProjectSlug:      "test-proj",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test-key"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "my-sentry-org",
					},
				},
			},
			wantClientKeys: []*sentry.ClientKey{},
			wantKubeClientKey: &sentryv1alpha1.ClientKey{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: nil,
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := &reconcilerSet{
				scheme: scheme.Scheme,
				kube:   fake.NewFakeClient(tc.kube...),
				sentry: tc.sentry,
			}

			_, err := r.ClientKey(tc.req)

			if tc.wantErr == nil && err != nil {
				t.Fatalf("want err to be nil, got: %q", err)
			}

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("want err %q, got: %q", tc.wantErr, err)
				}
				if !strings.Contains(err.Error(), tc.wantErr.Error()) {
					t.Fatalf("want err %q, got: %q", tc.wantErr, err)
				}
			}

			if want, got := len(tc.wantClientKeys), len(tc.sentry.ClientKeys); want != got {
				t.Fatalf("want %d key(s) on sentry, got: %d", want, got)
			}

			for i, want := range tc.wantClientKeys {
				got := tc.sentry.ClientKeys[i]

				if want.ID != got.ID {
					t.Fatalf("want client key #%d id %q, got: %q", i, want.ID, got.ID)
				}

				if want.Name != got.Name {
					t.Fatalf("want client key #%d label %q, got: %q", i, want.Name, got.Name)
				}
			}

			if want := tc.wantKubeClientKey; want != nil {
				got := &sentryv1alpha1.ClientKey{}
				err := r.kube.Get(
					context.TODO(),
					client.ObjectKey{Namespace: want.ObjectMeta.Namespace, Name: want.ObjectMeta.Name},
					got,
				)
				if err != nil {
					t.Fatal(err)
				}
				if got.Status.ID != want.Status.ID {
					t.Fatalf("want status.id %s, got: %s", want.Status.ID, got.Status.ID)
				}
				if got.Status.ProjectSlug != want.Status.ProjectSlug {
					t.Errorf("want status.team %q, got: %q", want.Status.ProjectSlug, got.Status.ProjectSlug)
				}
				if got.Status.OrganizationSlug != want.Status.OrganizationSlug {
					t.Errorf("want status.org %q, got: %q", want.Status.OrganizationSlug, got.Status.OrganizationSlug)
				}
				if !reflect.DeepEqual(got.ObjectMeta.Finalizers, want.ObjectMeta.Finalizers) {
					t.Errorf("want finalizers %+v, got: %+v", want.ObjectMeta.Finalizers, got.ObjectMeta.Finalizers)
				}
			}

			for _, want := range tc.wantKubeSecrets {
				got := &corev1.Secret{}
				err := r.kube.Get(
					context.TODO(),
					client.ObjectKey{Namespace: want.ObjectMeta.Namespace, Name: want.ObjectMeta.Name},
					got,
				)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(want.Data, got.Data) {
					t.Fatalf("want secret Data %+v, got: %+v", want.Data, got.Data)
				}
			}
		})
	}
}

func TestTeamReconciler(t *testing.T) {
	if err := sentryv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatal(err)
	}

	sentryTeam := &sentryv1alpha1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testing",
		},
		Spec: sentryv1alpha1.TeamSpec{
			Slug:             "testing",
			OrganizationSlug: "test-sentry-org",
		},
	}

	for _, tc := range []struct {
		name   string
		kube   []runtime.Object
		sentry *sentry.Fake
		req    reconcile.Request

		wantErr         error
		wantSentryTeams []*sentry.Team
		wantKubeTeam    *sentryv1alpha1.Team
	}{
		{
			name: "object is not found",
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "not-found", Name: "not-found"},
			},
			sentry:  &sentry.Fake{},
			wantErr: nil,
		},
		{
			name: "errors if organization does not exist",
			kube: []runtime.Object{sentryTeam},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry:  &sentry.Fake{},
			wantErr: errors.New("failed to create team"),
		},
		{
			name: "creates sentry team",
			kube: []runtime.Object{
				&sentryv1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "testing",
					},
					Spec: sentryv1alpha1.TeamSpec{
						Slug:             "test-team",
						OrganizationSlug: "test-org",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "test-org",
					},
				},
			},
			wantSentryTeams: []*sentry.Team{
				{
					Slug: "test-team",
					Name: "Test Team",
				},
			},
			wantKubeTeam: &sentryv1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  "testing",
					Name:       "test",
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.TeamStatus{
					Slug:             "test-team",
					OrganizationSlug: "test-org",
				},
			},
		},
		{
			name: "updates sentry team slug",
			kube: []runtime.Object{
				&sentryv1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "team",
					},
					Spec: sentryv1alpha1.TeamSpec{
						OrganizationSlug: "test-org",
						Slug:             "new-slug",
					},
					Status: sentryv1alpha1.TeamStatus{
						OrganizationSlug: "test-org",
						Slug:             "old-slug",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "team"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "test-org",
					},
				},
				Teams: []*sentry.Team{
					{
						Slug: "old-slug",
					},
				},
			},
			wantSentryTeams: []*sentry.Team{
				{
					Slug: "new-slug",
				},
			},
			wantKubeTeam: &sentryv1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  "testing",
					Name:       "team",
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.TeamStatus{
					Slug:             "new-slug",
					OrganizationSlug: "test-org",
				},
			},
		},
		{
			name: "deletes sentry team",
			kube: []runtime.Object{
				&sentryv1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "testing",
						Name:              "test-team",
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
						Finalizers:        []string{finalizerName},
					},
					Spec: sentryv1alpha1.TeamSpec{
						Slug:             "test-team",
						OrganizationSlug: "test-org",
					},
					Status: sentryv1alpha1.TeamStatus{
						Slug:             "test-team",
						OrganizationSlug: "test-org",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test-team"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "test-org",
					},
				},
				Teams: []*sentry.Team{
					{
						Slug: "test-team",
					},
					{
						Slug: "other-team",
					},
				},
			},
			wantSentryTeams: []*sentry.Team{
				{
					Slug: "other-team",
				},
			},
			wantKubeTeam: &sentryv1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  "testing",
					Name:       "test-team",
					Finalizers: nil,
				},
				Status: sentryv1alpha1.TeamStatus{},
			},
		},
		{
			name: "deletes noop when team does not exist",
			kube: []runtime.Object{
				&sentryv1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "testing",
						Name:              "test-team",
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
						Finalizers:        []string{finalizerName},
					},
					Spec: sentryv1alpha1.TeamSpec{
						Slug: "test-team",
					},
					Status: sentryv1alpha1.TeamStatus{
						Slug: "test-team",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test-team"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "my-sentry-org",
					},
				},
			},
			wantSentryTeams: []*sentry.Team{},
			wantKubeTeam: &sentryv1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  "testing",
					Name:       "test-team",
					Finalizers: nil,
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := &reconcilerSet{
				scheme: scheme.Scheme,
				kube:   fake.NewFakeClient(tc.kube...),
				sentry: tc.sentry,
			}

			_, err := r.Team(tc.req)

			if tc.wantErr == nil && err != nil {
				t.Fatalf("want err to be nil, got: %q", err)
			}

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("want err %q, got: %q", tc.wantErr, err)
				}
				if !strings.Contains(err.Error(), tc.wantErr.Error()) {
					t.Fatalf("want err %q, got: %q", tc.wantErr, err)
				}
			}

			if want, got := len(tc.wantSentryTeams), len(tc.sentry.Teams); want != got {
				t.Fatalf("want %d team(s) on sentry, got: %d", want, got)
			}

			for i, want := range tc.wantSentryTeams {
				got := tc.sentry.Teams[i]

				if want.Slug != got.Slug {
					t.Fatalf("want team #%d slug %q, got: %q", i, want.Slug, got.Slug)
				}
			}

			if want := tc.wantKubeTeam; want != nil {
				got := &sentryv1alpha1.Team{}
				err := r.kube.Get(
					context.TODO(),
					client.ObjectKey{Namespace: want.ObjectMeta.Namespace, Name: want.ObjectMeta.Name},
					got,
				)
				if err != nil {
					t.Fatal(err)
				}
				if got.Status.Slug != want.Status.Slug {
					t.Errorf("want status.Slug %q, got: %q", want.Status.Slug, got.Status.Slug)
				}
				if got.Status.OrganizationSlug != want.Status.OrganizationSlug {
					t.Errorf("want status.org %q, got: %q", want.Status.OrganizationSlug, got.Status.OrganizationSlug)
				}
				if !reflect.DeepEqual(got.ObjectMeta.Finalizers, want.ObjectMeta.Finalizers) {
					t.Errorf("want finalizers %+v, got: %+v", want.ObjectMeta.Finalizers, got.ObjectMeta.Finalizers)
				}
			}
		})
	}
}

func TestProjectReconciler(t *testing.T) {
	if err := sentryv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatal(err)
	}

	testProject := &sentryv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testing",
		},
		Spec: sentryv1alpha1.ProjectSpec{
			Slug:             "my-test-project",
			OrganizationSlug: "test-org",
			TeamSlug:         "team",
		},
	}

	for _, tc := range []struct {
		name   string
		kube   []runtime.Object
		sentry *sentry.Fake
		req    reconcile.Request

		wantErr         error
		wantProjects    []*sentry.Project
		wantKubeProject *sentryv1alpha1.Project
	}{
		{
			name: "object is not found",
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "not-found", Name: "not-found"},
			},
			sentry:  &sentry.Fake{},
			wantErr: nil,
		},
		{
			name: "errors if organization does not exist",
			kube: []runtime.Object{testProject},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry:  &sentry.Fake{},
			wantErr: errors.New("organization not found"),
		},
		{
			name: "errors if team does not exist",
			kube: []runtime.Object{
				&sentryv1alpha1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "testing",
					},
					Spec: sentryv1alpha1.ProjectSpec{
						Slug:     "my-test-project",
						TeamSlug: "team-not-found",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "my-sentry-org",
					},
				},
			},
			wantErr: errors.New("failed to create project"),
		},
		{
			name: "creates sentry project",
			kube: []runtime.Object{
				&sentryv1alpha1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "testing",
					},
					Spec: sentryv1alpha1.ProjectSpec{
						Slug:             "my-test-project",
						TeamSlug:         "my-team",
						OrganizationSlug: "my-org",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "my-org",
					},
				},
				Teams: []*sentry.Team{
					{
						Slug: "my-team",
					},
				},
			},
			wantProjects: []*sentry.Project{
				{
					Slug: "my-test-project",
					Name: "My Test Project",
				},
			},
			wantKubeProject: &sentryv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.ProjectStatus{
					Slug:             "my-test-project",
					TeamSlug:         "my-team",
					OrganizationSlug: "my-org",
				},
			},
		},
		{
			name: "updates sentry project slug",
			kube: []runtime.Object{
				&sentryv1alpha1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "test",
					},
					Spec: sentryv1alpha1.ProjectSpec{
						OrganizationSlug: "org",
						TeamSlug:         "my-team",
						Slug:             "new-slug",
					},
					Status: sentryv1alpha1.ProjectStatus{
						OrganizationSlug: "org",
						TeamSlug:         "my-team",
						Slug:             "old-slug",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "org",
					},
				},
				Teams: []*sentry.Team{
					{
						Slug: "my-team",
					},
				},
				Projects: []*sentry.Project{
					{
						Slug: "old-slug",
						Name: "My Name",
					},
				},
			},
			wantProjects: []*sentry.Project{
				{
					Slug: "new-slug",
					Name: "My Test Project",
				},
			},
			wantKubeProject: &sentryv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.ProjectStatus{
					Slug:             "new-slug",
					TeamSlug:         "my-team",
					OrganizationSlug: "org",
				},
			},
		},
		{
			name: "deletes sentry project",
			kube: []runtime.Object{
				&sentryv1alpha1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "testing",
						Name:              "test",
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
						Finalizers:        []string{finalizerName},
					},
					Spec: sentryv1alpha1.ProjectSpec{
						Slug:             "my-test-project",
						TeamSlug:         "test",
						OrganizationSlug: "test-org",
					},
					Status: sentryv1alpha1.ProjectStatus{
						Slug:             "my-test-project",
						TeamSlug:         "test",
						OrganizationSlug: "test-org",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "test-org",
					},
				},
				Teams: []*sentry.Team{
					{
						Slug: "my-team",
					},
				},
				Projects: []*sentry.Project{
					{
						Slug: "other-project",
						Name: "Other Project",
					},
					{
						Slug: "my-test-project",
						Name: "My Team",
					},
				},
			},
			wantProjects: []*sentry.Project{
				{
					Slug: "other-project",
					Name: "Other Project",
				},
			},
			wantKubeProject: &sentryv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: nil,
				},
				Status: sentryv1alpha1.ProjectStatus{},
			},
		},
		{
			name: "delete noops when project has already been deleted",
			kube: []runtime.Object{
				&sentryv1alpha1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         "testing",
						Name:              "test",
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
						Finalizers:        []string{finalizerName},
					},
					Status: sentryv1alpha1.ProjectStatus{
						Slug: "my-project",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &sentry.Fake{
				Orgs: []*sentry.Organization{
					{
						Slug: "my-sentry-org",
					},
				},
			},
			wantProjects: []*sentry.Project{},
			wantKubeProject: &sentryv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: nil,
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := &reconcilerSet{
				scheme: scheme.Scheme,
				kube:   fake.NewFakeClient(tc.kube...),
				sentry: tc.sentry,
			}

			_, err := r.Project(tc.req)

			if tc.wantErr == nil && err != nil {
				t.Fatalf("want err to be nil, got: %q", err)
			}

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("want err %q, got: %q", tc.wantErr, err)
				}
				if !strings.Contains(err.Error(), tc.wantErr.Error()) {
					t.Fatalf("want err %q, got: %q", tc.wantErr, err)
				}
			}

			if want, got := len(tc.wantProjects), len(tc.sentry.Projects); want != got {
				t.Fatalf("want %d project(s) on sentry, got: %d", want, got)
			}

			for i, want := range tc.wantProjects {
				got := tc.sentry.Projects[i]

				if want.Slug != got.Slug {
					t.Fatalf("want project #%d slug %q, got: %q", i, want.Slug, got.Slug)
				}
			}

			if want := tc.wantKubeProject; want != nil {
				got := &sentryv1alpha1.Project{}
				err := r.kube.Get(
					context.TODO(),
					client.ObjectKey{Namespace: want.ObjectMeta.Namespace, Name: want.ObjectMeta.Name},
					got,
				)
				if err != nil {
					t.Fatal(err)
				}
				if got.Status.Slug != want.Status.Slug {
					t.Errorf("want status.slug %q, got: %q", want.Status.Slug, got.Status.Slug)
				}
				if got.Status.TeamSlug != want.Status.TeamSlug {
					t.Errorf("want status.team %q, got: %q", want.Status.TeamSlug, got.Status.TeamSlug)
				}
				if got.Status.OrganizationSlug != want.Status.OrganizationSlug {
					t.Errorf("want status.org %q, got: %q", want.Status.OrganizationSlug, got.Status.OrganizationSlug)
				}
				if !reflect.DeepEqual(got.ObjectMeta.Finalizers, want.ObjectMeta.Finalizers) {
					t.Errorf("want finalizers %+v, got: %+v", want.ObjectMeta.Finalizers, got.ObjectMeta.Finalizers)
				}
			}
		})
	}
}

func TestHasFinalizer(t *testing.T) {
	for i, tc := range []struct {
		obj  metav1.Object
		want bool
	}{
		{
			obj:  &corev1.Pod{},
			want: false,
		},
		{
			obj: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{finalizerName},
			}},
			want: true,
		},
		{
			obj: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"foo"},
			}},
			want: false,
		},
	} {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Parallel()
			if want, got := tc.want, hasFinalizer(tc.obj); want != got {
				t.Errorf("want hasFinalizer %+v, got: %+v", want, got)
			}
		})
	}
}

func TestRemoveFinalizer(t *testing.T) {
	for i, tc := range []struct {
		obj  metav1.Object
		want []string
	}{
		{
			obj:  &corev1.Pod{},
			want: nil,
		},
		{
			obj: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{finalizerName},
			}},
			want: nil,
		},
		{
			obj: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{finalizerName, "foo"},
			}},
			want: []string{"foo"},
		},
		{
			obj: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{finalizerName, finalizerName},
			}},
			want: nil,
		},
	} {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Parallel()

			removeFinalizer(tc.obj)

			if want, got := tc.want, tc.obj.GetFinalizers(); !reflect.DeepEqual(want, got) {
				t.Errorf("want finalizers %+v, got: %+v", want, got)
			}
		})
	}
}
