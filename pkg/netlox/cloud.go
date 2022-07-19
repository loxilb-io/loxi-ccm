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
	"io"
	"net/url"

	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"loxi-ccm/pkg/api"
	"loxi-ccm/pkg/ippool"
)

const (
	LoxiProviderName = "netlox"
)

type LoxiClient struct {
	LoxiProviderName string
	LoxiVersion      string
	APIServerURL     *url.URL
	ExternalIPPool   *ippool.IPPool

	RESTClient *api.RESTClient
	k8sClient  kubernetes.Interface
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping or run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (l *LoxiClient) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	l.RESTClient = api.CreateRESTClient()
	l.k8sClient = clientBuilder.ClientOrDie("loxi-cloud-controller-manager")

	// Get all loadbalancer service in all namespace
	/*
		svcList, err := l.k8sClient.CoreV1().Services("").List(context.TODO(), v1.ListOptions{})
		if err != nil {
			klog.Errorf("Failed to initialize when get k8s services. err :%s", err.Error())
			return
		}

		klog.Infof("LoxiClient.Initialize: ")
		for _, svc := range svcList.Items {
			if svc.Spec.Type != "LoadBalancer" {
				continue
			}
			klog.Infof("type LoadBalancer service name: %s, namespace: %s", svc.Name, svc.Namespace)
		}
	*/
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (l *LoxiClient) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return l, true
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (l *LoxiClient) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

// InstancesV2 is an implementation for instances and should only be implemented by external cloud providers.
// Implementing InstancesV2 is behaviorally identical to Instances but is optimized to significantly reduce
// API calls to the cloud provider when registering and syncing nodes. Implementation of this interface will
// disable calls to the Zones interface. Also returns true if the interface is supported, false otherwise.
func (l *LoxiClient) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
// DEPRECATED: Zones is deprecated in favor of retrieving zone/region information from InstancesV2.
// This interface will not be called if InstancesV2 is enabled.
func (l *LoxiClient) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (l *LoxiClient) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (l *LoxiClient) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (l *LoxiClient) ProviderName() string {
	klog.V(5).Infof("LoxiClient.ProviderName() returned %s", l.LoxiProviderName)
	return l.LoxiProviderName
}

// HasClusterID returns true if a ClusterID is required and set
func (l *LoxiClient) HasClusterID() bool {
	klog.V(5).Info("LoxiClient.HasClusterID() returned true")
	return true
}

// io.Reader에 CCM config file 읽은 내용 전송되어서 옴.
// config file에는 일단 LoxiLB API server 접속에 필요한 server ip, port 정보가 있다고 생각하겠음.
func init() {
	cloudprovider.RegisterCloudProvider(LoxiProviderName, func(i io.Reader) (cloudprovider.Interface, error) {
		/*
			o := ReadLoxiConfigFile(i)
			return &LoxiClient{
				ApiServerIP:      o.ApiServerIP,
				ApiServerPort:    o.ApiServerPort,
				LoxiProviderName: LoxiProviderName,
			}, nil
		*/
		/*
			configByte, err := io.ReadAll(i)
			if err != nil {
				klog.Errorf("Failed to read config file when cloud provider(%s) regist", LoxiProviderName)
				return nil, err
			}

			o, err := ReadLoxiConfigFile(configByte)
			if err != nil {
				klog.Errorf("Failed to unmarshal config file for cloud provider(%s) regist", LoxiProviderName)
				return nil, err
			}
		*/
		o, err := ReadLoxiCCMEnvronment()
		if err != nil {
			klog.Errorf("loxi-ccm: failed to get environment")
			return nil, err
		}

		ipPool, err := ippool.NewIPPool(o.ExternalCIDR)
		if err != nil {
			klog.Errorf("loxi-ccm: failed to create external IP Pool (CIDR: %s)", o.ExternalCIDR)
			return nil, err
		}

		return &LoxiClient{
			LoxiProviderName: LoxiProviderName,
			LoxiVersion:      "v1",
			APIServerURL:     o.APIServerURL,
			ExternalIPPool:   ipPool,
		}, nil
	})
}
