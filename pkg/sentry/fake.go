package sentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var _ Client = &Fake{}

// Fake is a fake implementation of the Client interface.
type Fake struct {
	Orgs       []*Organization
	Teams      []*Team
	Projects   []*Project
	ClientKeys []*ClientKey
}

func (s *Fake) GetOrganization(ctx context.Context, slug string) (*Organization, *http.Response, error) {
	for _, org := range s.Orgs {
		if org.Slug == slug {
			return org, &http.Response{StatusCode: http.StatusOK}, nil
		}
	}
	return nil, &http.Response{StatusCode: http.StatusNotFound}, errors.New("not found")
}

func (s *Fake) GetTeam(ctx context.Context, org, slug string) (*Team, *http.Response, error) {
	for _, t := range s.Teams {
		if t.Slug == slug {
			return t, nil, nil
		}
	}
	return nil, &http.Response{StatusCode: http.StatusNotFound}, errors.New("found found")
}

func (s *Fake) CreateTeam(ctx context.Context, org, name, slug string) (*Team, *http.Response, error) {
	if slug == "" {
		s := strings.ToLower(name)
		s = strings.Replace(s, " ", "-", -1)
		slug = s
	}
	t := &Team{Name: name, Slug: slug}
	s.Teams = append(s.Teams, t)
	return t, nil, nil
}

func (s *Fake) UpdateProjectName(ctx context.Context, org, slug, name string) (*http.Response, error) {
	for _, p := range s.Projects {
		if p.Slug == slug {
			p.Name = name
			return &http.Response{StatusCode: http.StatusOK}, nil
		}
	}
	return &http.Response{StatusCode: http.StatusNotFound}, errors.New("found found")
}

func (s *Fake) UpdateTeamName(ctx context.Context, org, slug, name string) (*http.Response, error) {
	for _, t := range s.Teams {
		if t.Slug == slug {
			t.Name = name
			return &http.Response{StatusCode: http.StatusOK}, nil
		}
	}
	return &http.Response{StatusCode: http.StatusNotFound}, errors.New("not fond")
}

func (s *Fake) DeleteTeam(ctx context.Context, org, slug string) (*http.Response, error) {
	var found bool
	for _, t := range s.Teams {
		if t.Slug == slug {
			found = true
			break
		}
	}
	if !found {
		return &http.Response{StatusCode: http.StatusNotFound}, errors.New("not fond")
	}

	teams := []*Team{}
	for _, t := range s.Teams {
		if t.Slug != slug {
			teams = append(teams, t)
		}
	}
	s.Teams = teams
	return &http.Response{StatusCode: http.StatusNoContent}, nil
}

func (s *Fake) GetProject(ctx context.Context, org, slug string) (*Project, *http.Response, error) {
	for _, p := range s.Projects {
		if p.Slug == slug {
			return p, &http.Response{StatusCode: http.StatusOK}, nil
		}
	}
	return nil, &http.Response{StatusCode: http.StatusNotFound}, errors.New("not fond")
}

func (s *Fake) CreateClientKey(ctx context.Context, org, proj, name string) (*ClientKey, *http.Response, error) {
	var found bool
	for _, p := range s.Projects {
		if p.Slug == proj {
			found = true
			break
		}
	}
	if !found {
		return nil, &http.Response{StatusCode: http.StatusNotFound}, errors.New("not fond")
	}
	k := &ClientKey{
		ID:   fmt.Sprintf("%d", (len(s.ClientKeys) + 1)),
		Name: name,
		DSN: &ClientKeyDSN{
			Secret: "secret",
			CSP:    "csp",
			Public: "public",
		},
	}
	s.ClientKeys = append(s.ClientKeys, k)
	return k, &http.Response{StatusCode: http.StatusOK}, nil
}

func (s *Fake) GetClientKeys(ctx context.Context, org, proj string) ([]*ClientKey, *http.Response, error) {
	var found bool
	for _, p := range s.Projects {
		if p.Slug == proj {
			found = true
			break
		}
	}
	if !found {
		return nil, &http.Response{StatusCode: http.StatusNotFound}, errors.New("not found")
	}
	return s.ClientKeys, nil, nil
}

func (s *Fake) UpdateClientKeyName(ctx context.Context, org, proj, id, name string) (*http.Response, error) {
	var found bool
	for _, p := range s.Projects {
		if p.Slug == proj {
			found = true
			break
		}
	}
	if !found {
		return &http.Response{StatusCode: http.StatusNotFound}, errors.New("not found")
	}
	for _, k := range s.ClientKeys {
		if k.ID == id {
			k.Name = name
			return &http.Response{StatusCode: http.StatusOK}, nil
		}
	}
	return &http.Response{StatusCode: http.StatusNotFound}, errors.New("not found")
}

func (s *Fake) DeleteClientKey(ctx context.Context, org, proj, id string) (*http.Response, error) {
	var found bool
	for _, k := range s.ClientKeys {
		if k.ID == id {
			found = true
			break
		}
	}
	if !found {
		return &http.Response{StatusCode: http.StatusNotFound}, errors.New("not fond")
	}

	var keys []*ClientKey
	for _, k := range s.ClientKeys {
		if k.ID != id {
			keys = append(keys, k)
		}
	}
	s.ClientKeys = keys
	return &http.Response{StatusCode: http.StatusOK}, nil
}

func (s *Fake) CreateProject(ctx context.Context, org, team, name, slug string) (*Project, *http.Response, error) {
	if slug == "" {
		slug = strings.ToLower(name)
		slug = strings.Replace(slug, " ", "-", -1)
	}
	p := &Project{Name: name, Slug: slug}
	s.Projects = append(s.Projects, p)
	return p, &http.Response{StatusCode: http.StatusCreated}, nil
}

func (s *Fake) DeleteProject(ctx context.Context, org, slug string) (*http.Response, error) {
	var found bool
	for _, p := range s.Projects {
		if p.Slug == slug {
			found = true
			break
		}
	}
	if !found {
		return &http.Response{StatusCode: http.StatusNotFound}, errors.New("not fond")
	}

	var projs []*Project
	for _, p := range s.Projects {
		if p.Slug != slug {
			projs = append(projs, p)
		}
	}
	s.Projects = projs
	return &http.Response{StatusCode: http.StatusOK}, nil
}
