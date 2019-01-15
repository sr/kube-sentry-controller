package sentry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Client interface {
	GetOrganization(ctx context.Context, slug string) (*Organization, *http.Response, error)

	GetTeam(ctx context.Context, org, slug string) (*Team, *http.Response, error)
	CreateTeam(ctx context.Context, org, name, slug string) (*Team, *http.Response, error)
	UpdateTeam(ctx context.Context, org, slug, newName, newSlug string) (*Team, *http.Response, error)
	DeleteTeam(ctx context.Context, org, slug string) (*http.Response, error)

	GetProject(ctx context.Context, org, slug string) (*Project, *http.Response, error)
	CreateProject(ctx context.Context, org, team, name, slug string) (*Project, *http.Response, error)
	UpdateProject(ctx context.Context, org, slug, newName, newSlug string) (*Project, *http.Response, error)
	DeleteProject(ctx context.Context, org, slug string) (*http.Response, error)

	GetClientKeys(ctx context.Context, org, proj string) ([]*ClientKey, *http.Response, error)
	CreateClientKey(ctx context.Context, org, proj, name string) (*ClientKey, *http.Response, error)
	UpdateClientKey(ctx context.Context, org, proj, id, name string) (*http.Response, error)
	DeleteClientKey(ctx context.Context, org, proj, id string) (*http.Response, error)
}

type Organization struct {
	Slug string `json:"slug"`
}

type Team struct {
	Slug string `json:"slug,omitempty"`
	Name string `json:"name,omitempty"`
}

type Project struct {
	Slug string `json:"slug,omitempty"`
	Name string `json:"name,omitempty"`
}

type ClientKey struct {
	ID   string        `json:"id"`
	Name string        `json:"name"`
	DSN  *ClientKeyDSN `json:"dsn"`
}

type ClientKeyDSN struct {
	Secret string `json:"secret"`
	Public string `json:"public"`
	CSP    string `json:"csp"`
}

type ErrorResponse struct {
	Response *http.Response
	Body     []byte
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d %s",
		e.Response.Request.Method,
		e.Response.Request.URL,
		e.Response.StatusCode,
		string(e.Body),
	)
}

type httpClient struct {
	http    *http.Client
	baseURL *url.URL
}

func New(http *http.Client, baseURL *url.URL) Client {
	return &httpClient{http: http, baseURL: baseURL}
}

// https://docs.sentry.io/api/organizations/get-organization-details/
func (c *httpClient) GetOrganization(ctx context.Context, slug string) (*Organization, *http.Response, error) {
	req, err := c.newRequest(http.MethodGet, fmt.Sprintf("organizations/%s/", slug), nil)
	if err != nil {
		return nil, nil, err
	}
	org := &Organization{}
	resp, err := c.do(ctx, req, org)
	if err != nil {
		return nil, resp, err
	}
	return org, resp, nil
}

// https://docs.sentry.io/api/teams/get-team-details/
func (c *httpClient) GetTeam(ctx context.Context, org, slug string) (*Team, *http.Response, error) {
	req, err := c.newRequest(http.MethodGet, fmt.Sprintf("teams/%s/%s/", org, slug), nil)
	if err != nil {
		return nil, nil, err
	}
	team := &Team{}
	resp, err := c.do(ctx, req, team)
	if err != nil {
		return nil, resp, err
	}
	return team, resp, nil
}

// https://docs.sentry.io/api/teams/post-organization-teams/
func (c *httpClient) CreateTeam(ctx context.Context, org, name, slug string) (*Team, *http.Response, error) {
	req, err := c.newRequest(
		http.MethodPost,
		fmt.Sprintf("organizations/%s/teams/", org),
		Team{Name: name, Slug: slug},
	)
	if err != nil {
		return nil, nil, err
	}
	team := &Team{}
	resp, err := c.do(ctx, req, team)
	if err != nil {
		return nil, resp, err
	}
	return team, resp, nil
}

// https://docs.sentry.io/api/teams/put-team-details/
func (c *httpClient) UpdateTeam(ctx context.Context, org, slug, newName, newSlug string) (*Team, *http.Response, error) {
	req, err := c.newRequest(
		http.MethodPut,
		fmt.Sprintf("teams/%s/%s/", org, slug),
		Team{Name: newName, Slug: newSlug},
	)
	if err != nil {
		return nil, nil, err
	}
	team := &Team{}
	resp, err := c.do(ctx, req, team)
	if err != nil {
		return nil, resp, err
	}
	return team, resp, nil
}

// https://docs.sentry.io/api/teams/delete-team-details/
func (c *httpClient) DeleteTeam(ctx context.Context, org, slug string) (*http.Response, error) {
	req, err := c.newRequest(http.MethodDelete, fmt.Sprintf("teams/%s/%s/", org, slug), nil)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, req, nil)
}

// https://docs.sentry.io/api/projects/get-project-details/
func (c *httpClient) GetProject(ctx context.Context, org, slug string) (*Project, *http.Response, error) {
	req, err := c.newRequest(http.MethodGet, fmt.Sprintf("projects/%s/%s/", org, slug), nil)
	if err != nil {
		return nil, nil, err
	}
	proj := &Project{}
	resp, err := c.do(ctx, req, proj)
	if err != nil {
		return nil, resp, err
	}
	return proj, resp, nil
}

// https://docs.sentry.io/api/teams/post-team-projects/
func (c *httpClient) CreateProject(ctx context.Context, org, team, name, slug string) (*Project, *http.Response, error) {
	req, err := c.newRequest(
		http.MethodPost,
		fmt.Sprintf("teams/%s/%s/projects/", org, team),
		Project{Slug: slug, Name: name},
	)
	if err != nil {
		return nil, nil, err
	}
	proj := &Project{}
	resp, err := c.do(ctx, req, proj)
	if err != nil {
		return nil, resp, err
	}
	return proj, resp, nil
}

// https://docs.sentry.io/api/projects/put-project-details/
func (c *httpClient) UpdateProject(ctx context.Context, org, slug, newName, newSlug string) (*Project, *http.Response, error) {
	req, err := c.newRequest(
		http.MethodPut,
		fmt.Sprintf("projects/%s/%s/", org, slug),
		Project{Name: newName, Slug: newSlug},
	)
	if err != nil {
		return nil, nil, err
	}
	proj := &Project{}
	resp, err := c.do(ctx, req, proj)
	if err != nil {
		return nil, resp, err
	}
	return proj, resp, nil
}

// https://docs.sentry.io/api/projects/delete-project-details/
func (c *httpClient) DeleteProject(ctx context.Context, org, slug string) (*http.Response, error) {
	req, err := c.newRequest(http.MethodDelete, fmt.Sprintf("projects/%s/%s/", org, slug), nil)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, req, nil)
}

// https://docs.sentry.io/api/projects/get-project-keys/
func (c *httpClient) GetClientKeys(ctx context.Context, org, proj string) ([]*ClientKey, *http.Response, error) {
	req, err := c.newRequest(http.MethodGet, fmt.Sprintf("projects/%s/%s/keys/", org, proj), nil)
	if err != nil {
		return nil, nil, err
	}
	keys := []*ClientKey{}
	resp, err := c.do(ctx, req, &keys)
	if err != nil {
		return nil, resp, err
	}
	return keys, resp, nil
}

// https://docs.sentry.io/api/projects/post-project-keys/
func (c *httpClient) CreateClientKey(ctx context.Context, org, proj, name string) (*ClientKey, *http.Response, error) {
	req, err := c.newRequest(
		http.MethodPost,
		fmt.Sprintf("projects/%s/%s/keys/", org, proj),
		ClientKey{Name: name},
	)
	if err != nil {
		return nil, nil, err
	}
	key := &ClientKey{}
	resp, err := c.do(ctx, req, key)
	if err != nil {
		return nil, resp, err
	}
	return key, resp, nil
}

// https://docs.sentry.io/api/projects/put-project-key-details/
func (c *httpClient) UpdateClientKey(ctx context.Context, org, proj, id, name string) (*http.Response, error) {
	req, err := c.newRequest(
		http.MethodPut,
		fmt.Sprintf("projects/%s/%s/keys/%s/", org, proj, id),
		ClientKey{Name: name},
	)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, req, nil)
}

// https://docs.sentry.io/api/projects/delete-project-key-details/
func (c *httpClient) DeleteClientKey(ctx context.Context, org, proj, id string) (*http.Response, error) {
	req, err := c.newRequest(http.MethodDelete, fmt.Sprintf("projects/%s/%s/keys/%s/", org, proj, id), nil)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, req, nil)
}

func (c *httpClient) do(ctx context.Context, req *http.Request, v interface{}) (*http.Response, error) {
	req = req.WithContext(ctx)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if !(resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusCreated ||
		resp.StatusCode == http.StatusNoContent) {
		s, _ := ioutil.ReadAll(resp.Body)
		return resp, &ErrorResponse{Response: resp, Body: s}
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func (c *httpClient) newRequest(method, urlStr string, body interface{}) (*http.Request, error) {
	u, err := c.baseURL.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if body != nil {
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return req, err
}
