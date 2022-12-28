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
package netlox

import (
	"fmt"
	"net/url"
	"os"

	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

type LoxiConfig struct {
	APIServerUrlStrList []string `yaml:"apiServerURL"`
	ExternalCIDR        string   `yaml:"externalCIDR"`
	SetBGP              bool     `yaml:"setBGP"`
	SetLBMode           int32    `yaml:"setLBMode"`
	APIServerUrlList    []*url.URL
}

func ReadLoxiConfig(configBytes []byte) (LoxiConfig, error) {
	o := LoxiConfig{}

	if err := yaml.Unmarshal(configBytes, &o); err != nil {
		return o, fmt.Errorf("failed to unmarshal config. err: %v", err)
	}

	for _, u := range o.APIServerUrlStrList {
		apiURL, err := url.Parse(u)
		if err != nil {
			return o, err
		}

		klog.Infof("add loxilb API server %s", u)
		o.APIServerUrlList = append(o.APIServerUrlList, apiURL)
	}
	return o, nil
}

func ReadLoxiCCMEnvronment() (LoxiConfig, error) {
	ccmConfigStr, ok := os.LookupEnv("LOXICCM_CONFIG")
	if !ok {
		return LoxiConfig{}, fmt.Errorf("not found LOXICCM_CONFIG env")
	}

	return ReadLoxiConfig([]byte(ccmConfigStr))
}
