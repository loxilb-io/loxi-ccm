/*
 * Copyright (c) 2022 NetLOX Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package api

import (
	"bytes"
	"context"
	"net/http"
)

type RESTClient struct {
	Client *http.Client
}

func CreateRESTClient() *RESTClient {
	return &RESTClient{
		Client: &http.Client{},
	}
}

func (r *RESTClient) POST(ctx context.Context, postURL string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// move RESTOptions
	req.Header.Set("Content-Type", "application/json")
	return r.Client.Do(req)
}

func (r *RESTClient) DELETE(ctx context.Context, deleteURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return nil, err
	}

	// move RESTOptions
	req.Header.Set("Content-Type", "application/json")
	return r.Client.Do(req)
}

func (r *RESTClient) GET(ctx context.Context, getURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return r.Client.Do(req)
}
