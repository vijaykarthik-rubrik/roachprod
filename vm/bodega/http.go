package bodega

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

type httpClient struct {
	auth    string
	client  http.Client
}

func (c *httpClient) get(url *url.URL) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to create GET request for %s", url)
	}
	req.Header.Add("Authorization", c.auth)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to make GET request to %s", url)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf(
				"response status: %s\nunable to read response body",
				resp.Status,
			)
		}
		return nil, fmt.Errorf(
			"response status: %s\nresponse body:\n%v",
			resp.Status,
			string(body),
		)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *httpClient) post(url *url.URL, b []byte) (map[string]interface{}, error) {
	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to create POST request for %s", url)
	}
	req.Header.Add("Authorization", c.auth)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to make POST request to %s", url)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf(
				"response status: %s\nunable to read response body",
				resp.Status,
			)
		}
		return nil, fmt.Errorf(
			"response status: %s\nresponse body:\n%v",
			resp.Status,
			string(body),
		)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
