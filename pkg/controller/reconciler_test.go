package sentrycontroller

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	sentry "github.com/atlassian/go-sentry-api"
	sentryv1alpha1 "github.com/sr/kube-sentry-controller/pkg/apis/sentry/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	scheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func strP(s string) *string {
	return &s
}

func TestClientKeyReconciler(t *testing.T) {
	if err := sentryv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		t.Fatal(err)
	}

	sentryClientKey := &sentryv1alpha1.ClientKey{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "testing",
			Name:      "test-key",
		},
		Spec: sentryv1alpha1.ClientKeySpec{
			Name:    "My Key",
			Project: "my-project",
		},
	}

	for _, tc := range []struct {
		name   string
		kube   []runtime.Object
		sentry *fakeSentryClient
		req    reconcile.Request

		wantErr           error
		wantClientKeys    []sentry.Key
		wantKubeClientKey *sentryv1alpha1.ClientKey
		wantKubeSecrets   []*corev1.Secret
	}{
		{
			name: "object is not found",
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "not-found", Name: "not-found"},
			},
			sentry:  &fakeSentryClient{},
			wantErr: nil,
		},
		{
			name: "errors if organization does not exist",
			kube: []runtime.Object{sentryClientKey},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test-key"},
			},
			sentry:  &fakeSentryClient{},
			wantErr: errors.New("failed to get organization"),
		},
		{
			name: "errors if project does not exist",
			kube: []runtime.Object{sentryClientKey},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test-key"},
			},
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
			},
			wantErr: errors.New("failed to get project"),
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
						Name:    "My Key",
						Project: "my-project",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "sentry-key-1"},
			},
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
				projects: []sentry.Project{
					{
						Slug: strP("my-project"),
					},
				},
			},
			wantClientKeys: []sentry.Key{
				{
					ID:    "1",
					Label: "My Key",
				},
			},
			wantKubeClientKey: &sentryv1alpha1.ClientKey{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.ClientKeyStatus{
					ID:      "1",
					Project: "my-project",
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
						Name:    "new key name",
						Project: "my-project",
					},
					Status: sentryv1alpha1.ClientKeyStatus{
						ID:      "1",
						Project: "my-project",
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
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
				projects: []sentry.Project{
					{
						Slug: strP("my-project"),
					},
				},
				keys: []sentry.Key{
					{
						ID:    "1",
						Label: "old key name",
						DSN: sentry.DSN{
							Public: "new public",
							CSP:    "new csp",
							Secret: "new secret",
						},
					},
				},
			},
			wantClientKeys: []sentry.Key{
				{
					ID:    "1",
					Label: "new key name",
				},
			},
			wantKubeClientKey: &sentryv1alpha1.ClientKey{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.ClientKeyStatus{
					ID:      "1",
					Project: "my-project",
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
						Name:    "new key name",
						Project: "my-project",
					},
					Status: sentryv1alpha1.ClientKeyStatus{
						ID:      "1",
						Project: "my-project",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test-key"},
			},
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
				projects: []sentry.Project{
					{
						Slug: strP("my-project"),
					},
				},
				keys: []sentry.Key{
					{
						ID:    "1",
						Label: "key name",
						DSN: sentry.DSN{
							Public: "public",
							CSP:    "csp",
							Secret: "secret",
						},
					},
					{
						ID:    "2",
						Label: "some other key",
						DSN: sentry.DSN{
							Public: "public",
							CSP:    "csp",
							Secret: "secret",
						},
					},
				},
			},
			wantClientKeys: []sentry.Key{
				{
					ID:    "2",
					Label: "some other key",
				},
			},
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
				org:    "my-sentry-org",
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

			if want, got := len(tc.wantClientKeys), len(tc.sentry.keys); want != got {
				t.Fatalf("want %d key(s) on sentry, got: %d", want, got)
			}

			for i, want := range tc.wantClientKeys {
				got := tc.sentry.keys[i]

				if want.ID != got.ID {
					t.Fatalf("want client key #%d id %q, got: %q", i, want.ID, got.ID)
				}

				if want.Label != got.Label {
					t.Fatalf("want client key #%d label %q, got: %q", i, want.Label, got.Label)
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
				if got.Status.Project != want.Status.Project {
					t.Fatalf("want status.project %s, got: %s", want.Status.Project, got.Status.Project)
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
			Name: "Testing",
		},
	}

	for _, tc := range []struct {
		name   string
		kube   []runtime.Object
		sentry *fakeSentryClient
		req    reconcile.Request

		wantErr         error
		wantSentryTeams []sentry.Team
		wantKubeTeam    *sentryv1alpha1.Team
	}{
		{
			name: "object is not found",
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "not-found", Name: "not-found"},
			},
			sentry:  &fakeSentryClient{},
			wantErr: nil,
		},
		{
			name: "errors if organization does not exist",
			kube: []runtime.Object{sentryTeam},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry:  &fakeSentryClient{},
			wantErr: errors.New("failed to get organization"),
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
						Name: "Test Team",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
			},
			wantSentryTeams: []sentry.Team{
				{
					Slug: strP("test-team"),
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
					Slug: "test-team",
				},
			},
		},
		{
			name: "updates sentry team",
			kube: []runtime.Object{
				&sentryv1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "team",
					},
					Spec: sentryv1alpha1.TeamSpec{
						Name: "New Team Name",
					},
					Status: sentryv1alpha1.TeamStatus{
						Slug: "test-team",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "team"},
			},
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
				teams: []sentry.Team{
					{
						Slug: strP("test-team"),
						Name: "Old Team Name",
					},
				},
			},
			wantSentryTeams: []sentry.Team{
				{
					Slug: strP("test-team"),
					Name: "New Team Name",
				},
			},
			wantKubeTeam: &sentryv1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  "testing",
					Name:       "team",
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.TeamStatus{
					Slug: "test-team",
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
						Name: "Test Name",
					},
					Status: sentryv1alpha1.TeamStatus{
						Slug: "test-team",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test-team"},
			},
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
				teams: []sentry.Team{
					{
						Slug: strP("test-team"),
						Name: "Test Team",
					},
					{
						Slug: strP("other-team"),
						Name: "Other Team",
					},
				},
			},
			wantSentryTeams: []sentry.Team{
				{
					Slug: strP("other-team"),
					Name: "Other Team",
				},
			},
			wantKubeTeam: &sentryv1alpha1.Team{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  "testing",
					Name:       "test-team",
					Finalizers: nil,
				},
				Status: sentryv1alpha1.TeamStatus{
					Slug: "test-team",
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
				org:    "my-sentry-org",
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

			if want, got := len(tc.wantSentryTeams), len(tc.sentry.teams); want != got {
				t.Fatalf("want %d teams on sentry, got: %d", want, got)
			}

			for i, want := range tc.wantSentryTeams {
				got := tc.sentry.teams[i]

				if want.Name != got.Name {
					t.Fatalf("want team #%d name %q, got: %q", i, want.Name, got.Name)
				}
				if *want.Slug != *got.Slug {
					t.Fatalf("want team #%d slug %q, got: %q", i, *want.Slug, *got.Slug)
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
			Name: "My Test Project",
		},
	}

	for _, tc := range []struct {
		name   string
		kube   []runtime.Object
		sentry *fakeSentryClient
		req    reconcile.Request

		wantErr         error
		wantProjects    []sentry.Project
		wantKubeProject *sentryv1alpha1.Project
	}{
		{
			name: "object is not found",
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "not-found", Name: "not-found"},
			},
			sentry:  &fakeSentryClient{},
			wantErr: nil,
		},
		{
			name: "errors if organization does not exist",
			kube: []runtime.Object{testProject},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry:  &fakeSentryClient{},
			wantErr: errors.New("failed to get organization"),
		},
		{
			name: "errors if referenced team object not exist",
			kube: []runtime.Object{
				&sentryv1alpha1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "testing",
					},
					Spec: sentryv1alpha1.ProjectSpec{
						Name: "My Test Project",
						TeamRef: sentryv1alpha1.TeamReference{
							Namespace: "testing",
							Name:      "team-not-found",
						},
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
			},
			wantErr: errors.New("failed to get team referenced"),
		},
		{
			name: "errors if team does not exist",
			kube: []runtime.Object{
				&sentryv1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "test",
					},
					Spec: sentryv1alpha1.TeamSpec{
						Name: "Test Team",
					},
					Status: sentryv1alpha1.TeamStatus{
						Slug: "test-team",
					},
				},
				&sentryv1alpha1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "testing",
					},
					Spec: sentryv1alpha1.ProjectSpec{
						Name: "My Test Project",
						TeamRef: sentryv1alpha1.TeamReference{
							Namespace: "testing",
							Name:      "test",
						},
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
			},
			wantErr: errors.New("failed to get team my-sentry-org/test-team"),
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
						Name: "My Test Project",
						TeamRef: sentryv1alpha1.TeamReference{
							Namespace: "testing",
							Name:      "test",
						},
					},
				},
				&sentryv1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "test",
					},
					Spec: sentryv1alpha1.TeamSpec{
						Name: "My Test Project",
					},
					Status: sentryv1alpha1.TeamStatus{
						Slug: "my-team",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
				teams: []sentry.Team{
					{
						Slug: strP("my-team"),
					},
				},
			},
			wantProjects: []sentry.Project{
				{
					Slug: strP("my-test-project"),
					Name: "My Test Project",
					Team: &sentry.Team{
						Slug: strP("my-team"),
					},
				},
			},
			wantKubeProject: &sentryv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.ProjectStatus{
					Slug: "my-test-project",
					Team: "my-team",
				},
			},
		},
		{
			name: "updates sentry project",
			kube: []runtime.Object{
				&sentryv1alpha1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "testing",
						Name:      "test",
					},
					Spec: sentryv1alpha1.ProjectSpec{
						Name: "My Test Project",
						TeamRef: sentryv1alpha1.TeamReference{
							Namespace: "testing",
							Name:      "test",
						},
					},
					Status: sentryv1alpha1.ProjectStatus{
						Slug: "my-test-project",
					},
				},
				&sentryv1alpha1.Team{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "testing",
					},
					Spec: sentryv1alpha1.TeamSpec{
						Name: "My Test Project",
					},
					Status: sentryv1alpha1.TeamStatus{
						Slug: "my-team",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
				teams: []sentry.Team{
					{
						Slug: strP("my-team"),
					},
				},
				projects: []sentry.Project{
					{
						Slug: strP("my-test-project"),
						Name: "My Name",
						Team: &sentry.Team{
							Slug: strP("my-team"),
						},
					},
				},
			},
			wantProjects: []sentry.Project{
				{
					Slug: strP("my-test-project"),
					Name: "My Test Project",
					Team: &sentry.Team{
						Slug: strP("my-team"),
					},
				},
			},
			wantKubeProject: &sentryv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{finalizerName},
				},
				Status: sentryv1alpha1.ProjectStatus{
					Team: "my-team",
					Slug: "my-test-project",
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
						Name: "My Test Project",
						TeamRef: sentryv1alpha1.TeamReference{
							Namespace: "testing",
							Name:      "test",
						},
					},
					Status: sentryv1alpha1.ProjectStatus{
						Slug: "my-test-project",
						Team: "my-team",
					},
				},
			},
			req: reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: "testing", Name: "test"},
			},
			sentry: &fakeSentryClient{
				orgs: []sentry.Organization{
					{
						Slug: strP("my-sentry-org"),
					},
				},
				teams: []sentry.Team{
					{
						Slug: strP("my-team"),
					},
				},
				projects: []sentry.Project{
					{
						Slug: strP("other-project"),
						Name: "Other Project",
						Team: &sentry.Team{
							Slug: strP("my-team"),
						},
					},
					{
						Slug: strP("my-test-project"),
						Name: "My Team",
						Team: &sentry.Team{
							Slug: strP("my-team"),
						},
					},
				},
			},
			wantProjects: []sentry.Project{
				{
					Slug: strP("other-project"),
					Name: "Other Project",
					Team: &sentry.Team{
						Slug: strP("my-team"),
					},
				},
			},
			wantKubeProject: &sentryv1alpha1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: nil,
				},
				Status: sentryv1alpha1.ProjectStatus{},
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
				org:    "my-sentry-org",
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

			if want, got := len(tc.wantProjects), len(tc.sentry.projects); want != got {
				t.Fatalf("want %d project(s) on sentry, got: %d", want, got)
			}

			for i, want := range tc.wantProjects {
				got := tc.sentry.projects[i]

				if want.Name != got.Name {
					t.Fatalf("want project #%d name %q, got: %q", i, want.Name, got.Name)
				}
				if *want.Slug != *got.Slug {
					t.Fatalf("want project #%d slug %q, got: %q", i, *want.Slug, *got.Slug)
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
				if got.Status.Team != want.Status.Team {
					t.Errorf("want status.team %q, got: %q", want.Status.Team, got.Status.Team)
				}
				if !reflect.DeepEqual(got.ObjectMeta.Finalizers, want.ObjectMeta.Finalizers) {
					t.Errorf("want finalizers %+v, got: %+v", want.ObjectMeta.Finalizers, got.ObjectMeta.Finalizers)
				}
			}
		})
	}
}
