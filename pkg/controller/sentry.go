package controller

import (
	"fmt"
	"strings"

	sentry "github.com/atlassian/go-sentry-api"
)

// SentryClient is the subset of Atlassian's sentry.Client interface that is used by this package.
type SentryClient interface {
	GetOrganization(string) (sentry.Organization, error)

	CreateTeam(sentry.Organization, string, *string) (sentry.Team, error)
	GetTeam(sentry.Organization, string) (sentry.Team, error)
	UpdateTeam(sentry.Organization, sentry.Team) error
	DeleteTeam(sentry.Organization, sentry.Team) error

	GetProject(sentry.Organization, string) (sentry.Project, error)
	CreateProject(sentry.Organization, sentry.Team, string, *string) (sentry.Project, error)
	UpdateProject(sentry.Organization, sentry.Project) error
	DeleteProject(sentry.Organization, sentry.Project) error

	GetClientKeys(sentry.Organization, sentry.Project) ([]sentry.Key, error)
	CreateClientKey(sentry.Organization, sentry.Project, string) (sentry.Key, error)
	UpdateClientKey(sentry.Organization, sentry.Project, sentry.Key, string) (sentry.Key, error)
	DeleteClientKey(sentry.Organization, sentry.Project, sentry.Key) error
}

// fakeSentryClient is a fake implementation of the SentryClient interface.
type fakeSentryClient struct {
	orgs     []sentry.Organization
	teams    []sentry.Team
	projects []sentry.Project
	keys     []sentry.Key
}

func (s *fakeSentryClient) GetOrganization(slug string) (sentry.Organization, error) {
	for _, org := range s.orgs {
		if *org.Slug == slug {
			return org, nil
		}
	}
	return sentry.Organization{}, sentry.APIError{StatusCode: 404}
}

func (s *fakeSentryClient) GetTeam(org sentry.Organization, slug string) (sentry.Team, error) {
	for _, t := range s.teams {
		if *t.Slug == slug {
			return t, nil
		}
	}
	return sentry.Team{}, sentry.APIError{StatusCode: 404}
}

func (s *fakeSentryClient) CreateTeam(org sentry.Organization, name string, slug *string) (sentry.Team, error) {
	if slug == nil {
		s := strings.ToLower(name)
		s = strings.Replace(s, " ", "-", -1)
		slug = &s
	}
	t := sentry.Team{Name: name, Slug: slug}
	s.teams = append(s.teams, t)
	return t, nil
}

func (s *fakeSentryClient) UpdateTeam(org sentry.Organization, team sentry.Team) error {
	for i, t := range s.teams {
		if t.Slug == team.Slug {
			s.teams[i] = team
			return nil
		}
	}
	return sentry.APIError{StatusCode: 404}
}

func (s *fakeSentryClient) DeleteTeam(org sentry.Organization, team sentry.Team) error {
	var found bool
	for _, t := range s.teams {
		if t.Slug == team.Slug {
			found = true
			break
		}
	}
	if !found {
		return sentry.APIError{StatusCode: 404}
	}

	teams := []sentry.Team{}
	for _, t := range s.teams {
		if t.Slug != team.Slug {
			teams = append(teams, t)
		}
	}
	s.teams = teams
	return nil
}

func (s *fakeSentryClient) GetProject(org sentry.Organization, slug string) (sentry.Project, error) {
	for _, p := range s.projects {
		if *p.Slug == slug {
			return p, nil
		}
	}
	return sentry.Project{}, sentry.APIError{StatusCode: 404}
}

func (s *fakeSentryClient) CreateClientKey(org sentry.Organization, proj sentry.Project, label string) (sentry.Key, error) {
	k := sentry.Key{
		ID:    fmt.Sprintf("%d", (len(s.keys) + 1)),
		Label: label,
		DSN: sentry.DSN{
			Secret: "secret",
			CSP:    "csp",
			Public: "public",
		},
	}
	s.keys = append(s.keys, k)
	return k, nil
}

func (s *fakeSentryClient) GetClientKeys(org sentry.Organization, proj sentry.Project) ([]sentry.Key, error) {
	return s.keys, nil
}

func (s *fakeSentryClient) UpdateClientKey(org sentry.Organization, proj sentry.Project, key sentry.Key, label string) (sentry.Key, error) {
	for i, k := range s.keys {
		if k.ID == key.ID {
			s.keys[i].Label = label
			return s.keys[i], nil
		}
	}
	return sentry.Key{}, sentry.APIError{StatusCode: 404}
}

func (s *fakeSentryClient) DeleteClientKey(org sentry.Organization, proj sentry.Project, key sentry.Key) error {
	var found bool
	for _, k := range s.keys {
		if k.ID == key.ID {
			found = true
			break
		}
	}
	if !found {
		return sentry.APIError{StatusCode: 404}
	}

	keys := []sentry.Key{}
	for _, k := range s.keys {
		if k.ID != key.ID {
			keys = append(keys, k)
		}
	}
	s.keys = keys
	return nil
}

func (s *fakeSentryClient) CreateProject(org sentry.Organization, team sentry.Team, name string, slug *string) (sentry.Project, error) {
	if slug == nil {
		s := strings.ToLower(name)
		s = strings.Replace(s, " ", "-", -1)
		slug = &s
	}
	p := sentry.Project{Name: name, Slug: slug, Team: &team}
	s.projects = append(s.projects, p)
	return p, nil
}

func (s *fakeSentryClient) UpdateProject(org sentry.Organization, proj sentry.Project) error {
	for i, p := range s.projects {
		if p.Slug == proj.Slug {
			s.projects[i] = sentry.Project{
				Slug: proj.Slug,
				Name: proj.Name,
				Team: &sentry.Team{
					Name: p.Team.Name,
					Slug: p.Team.Slug,
				},
			}
			return nil
		}
	}
	return sentry.APIError{StatusCode: 404}
}

func (s *fakeSentryClient) DeleteProject(org sentry.Organization, proj sentry.Project) error {
	var found bool
	for _, k := range s.projects {
		if *k.Slug == *proj.Slug {
			found = true
			break
		}
	}
	if !found {
		return sentry.APIError{StatusCode: 404}
	}

	projs := []sentry.Project{}
	for _, p := range s.projects {
		if *p.Slug != *proj.Slug {
			projs = append(projs, p)
		}
	}
	s.projects = projs
	return nil
}
