/*
Copyright © 2022 Netlox Inc. <backguyn@netlox.io>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package netlox

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
)

type LoxiConfig struct {
	APIServerURLStr string `json:"apiServerURL"`
	ExternalCIDR    string `json:"externalCIDR"`
	SetBGP          bool   `json:"setBGP"`
	APIServerURL    *url.URL
}

func ReadLoxiConfigFile(configBytes []byte) (LoxiConfig, error) {
	o := LoxiConfig{}
	if err := json.Unmarshal(configBytes, &o); err != nil {
		return o, err
	}

	if o.APIServerURLStr != "" {
		apiURL, err := url.Parse(o.APIServerURLStr)
		if err != nil {
			return o, err
		}

		o.APIServerURL = apiURL
	}

	return o, nil
}

func ReadLoxiCCMEnvronment() (LoxiConfig, error) {
	o := LoxiConfig{}
	var ok bool
	var err error
	var setBGP string

	o.ExternalCIDR, ok = os.LookupEnv("LOXILB_EXTERNAL_CIDR")
	if !ok {
		return o, fmt.Errorf("not found LOXILB_EXTERNAL_CIDR env")
	}

	o.APIServerURLStr, ok = os.LookupEnv("LOXILB_API_SERVER")
	if !ok {
		return o, fmt.Errorf("not found LOXILB_API_SERVER env")
	}

	o.APIServerURL, err = url.Parse(o.APIServerURLStr)
	if err != nil {
		return o, err
	}

	setBGP, ok = os.LookupEnv("LOXILB_SET_BGP")
	if !ok {
		o.SetBGP = false
	} else {
		if setBGP == "true" {
			o.SetBGP = true
		} else {
			o.SetBGP = false
		}
	}

	return o, nil
}
