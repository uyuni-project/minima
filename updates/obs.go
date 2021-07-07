// Lib's Co-Author: Felix Gerling, fgerling@suse.com, https://github.com/fgerling
package updates

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func (c *Client) NewRequest(method, path string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.BaseURL.ResolveReference(rel)
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := xml.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Username, c.Password)
	if body != nil {
		req.Header.Set("Content-Type", "application/xml")
	}
	req.Header.Set("Accept", "application/xml")
	return req, nil
}

func (c *Client) GetReleaseRequests(group string, state string) ([]ReleaseRequest, error) {
	req, err := c.NewRequest("GET", "/request", nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("view", "collection")
	q.Add("group", group)
	q.Add("states", state)
	req.URL.RawQuery = q.Encode()

	var collection Collection
	_, err = c.do(req, &collection)
	return collection.ReleaseRequests, err
}

func (c *Client) GetPatchinfo(rr ReleaseRequest) (*Patchinfo, error) {
	project := rr.Actions[0].Source.Project
	patchinfo_url := fmt.Sprintf("/source/%v/patchinfo/_patchinfo", project)
	req, err := c.NewRequest("GET", patchinfo_url, nil)
	if err != nil {
		return nil, err
	}
	var patchinfo Patchinfo
	_, err = c.do(req, &patchinfo)
	return &patchinfo, err
}

func (c *Client) do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errors.New(fmt.Sprintf("Got status code: %v for %q\n", resp.StatusCode, req.URL))
	}
	defer resp.Body.Close()
	err = xml.NewDecoder(resp.Body).Decode(v)
	return resp, err
}

func NewClient(username string, password string) *Client {
	return &Client{
		BaseURL:    &url.URL{Host: baseUrl, Scheme: "https"},
		Username:   username,
		Password:   password,
		httpClient: &http.Client{},
	}
}

func CheckWebPageExists(repoURL string) (bool, error) {
 	client := &http.Client{}
	resp, err := client.Head(repoURL)
	if err != nil {
		return false, err
	}
	if resp.Status == "200 OK" {
		return true, nil 
	}
		return false, nil
}
