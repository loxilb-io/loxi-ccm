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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type LoadBalancerModel struct {
	Service   LoadBalancerService    `json:"serviceArguments"`
	Endpoints []LoadBalancerEndpoint `json:"endpoints"`
}

type LoadBalancerListModel struct {
	LoadBalancerList []LoadBalancerModel `json:"lbAttr"`
}

type LoadBalancerService struct {
	ExternalIP string   `json:"externalIP"`
	Port       int32    `json:"port"`
	Protocol   string   `json:"protocol"`
	Sel        EpSelect `json:"sel"`
}

type EpSelect uint

type LoadBalancerEndpoint struct {
	EndpointIP string `json:"endpointIP"`
	TargetPort int32  `json:"targetPort"`
	Weight     int8   `json:"weight"`
}

const (
	LoxiLoadBalancerResource = "config/loadbalancer"
	LoxiMaxWeight            = 10
	testExternalIP           = "17.17.17.17"
)

func (l *LoxiClient) GetLoxiLoadBalancerAPIUrlString(subResource []string) string {
	p := path.Join(l.LoxiProviderName, l.LoxiVersion, LoxiLoadBalancerResource)
	if subResource != nil && len(subResource) > 0 {
		subPath := path.Join(subResource...)
		p = path.Join(p, subPath)
	}

	lbURL := url.URL{
		Scheme: l.APIServerURL.Scheme,
		Host:   l.APIServerURL.Host,
		Path:   p,
	}

	return lbURL.String()
}

// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *LoxiClient) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	klog.Infof("Called LoadBalancer.GetLoadBalancer(). clusterName: %s", clusterName)
	klog.Infof(" # Service.type: %s", service.Spec.Type)
	klog.Infof(" # Service.Name: %s", service.Name)
	klog.Infof(" # Service.Namespace: %s", service.Namespace)

	subResource := []string{
		"all",
	}
	loxiGetLoadBalancerURL := l.GetLoxiLoadBalancerAPIUrlString(subResource)
	resp, err := l.RESTClient.GET(ctx, loxiGetLoadBalancerURL)
	if err != nil {
		klog.Errorf("failed to LoadBalancer.GetLoadBalancer() call to LoxiLB API. err: %s", err.Error())
		return nil, false, err
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		klog.Errorf("failed to read response body in LoadBalancer.GetLoadBalancer(). err: %s", err.Error())
		return nil, false, err
	}

	if resp.StatusCode != http.StatusOK {
		klog.Errorf("failed to get Load Balancer. LoxiLB return response code %d. message. %v", resp.StatusCode, respBody)
		return nil, false, fmt.Errorf("loxilb return response %d. %s", resp.StatusCode, string(respBody))
	}

	lbListModel := LoadBalancerListModel{}
	if err := json.Unmarshal(respBody, &lbListModel); err != nil {
		klog.Errorf("failed to unmarshal response body in LoadBalancer.GetLoadBalancer()")
		return nil, false, err
	}

	ingresses := service.Status.LoadBalancer.Ingress
	for _, lbModel := range lbListModel.LoadBalancerList {
		for _, ingress := range ingresses {
			klog.Infof("  ### Service.LoadBalancer.Ingress: %s == LoxiLB.ExternalIP: %s", ingress.IP, lbModel.Service.ExternalIP)
			if lbModel.Service.ExternalIP == ingress.IP {
				status := &v1.LoadBalancerStatus{}
				status.Ingress = []v1.LoadBalancerIngress{{IP: lbModel.Service.ExternalIP}}
				return status, true, nil
			}
		}
	}

	klog.Infof("not found Load Balancer (Ingresses: %v)", ingresses)
	return nil, false, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *LoxiClient) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	klog.Infof("Called LoadBalancer.GetLoadBalancerName(). clusterName: %s", clusterName)
	klog.Infof(" # Service.type: %s", service.Spec.Type)
	klog.Infof(" # Service.Name: %s", service.Name)
	klog.Infof(" # Service.Namespace: %s", service.Namespace)

	klog.Infof("LoadBalancer.GetLoadBalancerName() returned v1.service.Name: %s", service.Name)
	return service.Name
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *LoxiClient) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {

	klog.Infof("LoadBalancer.EnsureLoadBalancer() returned *v1.LoadBalancerStatus. corev1.LoadBalancerIngress{IP: %s}", testExternalIP)

	endpointIPs := l.getEndpointsForLB(nodes)
	loxiCreateLoadBalancerURL := l.GetLoxiLoadBalancerAPIUrlString(nil)

	for _, port := range service.Spec.Ports {
		lbModel := l.makeLoxiLoadBalancerModel(testExternalIP, port, endpointIPs)
		body, err := json.Marshal(lbModel)
		if err != nil {
			klog.Errorf("failed to EnsureLoadBalancer(). err: %s", err.Error())
			return nil, err
		}

		resp, err := l.RESTClient.POST(ctx, loxiCreateLoadBalancerURL, body)
		if err != nil {
			klog.Errorf("failed to LoadBalancer.EnsureLoadBalancer() call to LoxiLB API. err: %s", err.Error())
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			klog.Errorf("failed to LoadBalancer.EnsureLoadBalancer(): get response status %d.", resp.StatusCode)
			return nil, fmt.Errorf("loxiLB return response code %d. response.Body: %s", resp.StatusCode, string(respBody))
		}
	}

	status := &v1.LoadBalancerStatus{}
	status.Ingress = []v1.LoadBalancerIngress{{IP: testExternalIP}}
	return status, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *LoxiClient) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.Infof("Called LoadBalancer.UpdateLoadBalancer(). clusterName: %s", clusterName)
	klog.Infof(" # Service.type: %s", service.Spec.Type)
	klog.Infof(" # Service.Name: %s", service.Name)
	klog.Infof(" # Service.Namespace: %s", service.Namespace)
	for _, node := range nodes {
		klog.Infof(" ### Node.Name: %s", node.Name)
	}

	return nil
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still laying around.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *LoxiClient) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.Infof("Called LoadBalancer.EnsureLoadBalancerDeleted(). clusterName: %s", clusterName)
	klog.Infof(" # Service.type: %s", service.Spec.Type)
	klog.Infof(" # Service.Name: %s", service.Name)
	klog.Infof(" # Service.Namespace: %s", service.Namespace)

	ingresses := service.Status.LoadBalancer.Ingress
	ports := service.Spec.Ports
	for _, ingress := range ingresses {
		for _, port := range ports {
			// TODO: need to change argument?
			loxiDeleteLoadBalancerURL := l.GetLoxiLoadBalancerAPIUrlString([]string{
				"externalipaddress", ingress.IP,
				"port", strconv.Itoa(int(port.Port)),
				"protocol", strings.ToLower(string(port.Protocol)),
			})

			resp, err := l.RESTClient.DELETE(ctx, loxiDeleteLoadBalancerURL)
			// TODO:
			if err != nil {
				klog.Errorf("failed to call LoxiLB API. err: %s", err.Error())
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				respBody, _ := io.ReadAll(resp.Body)
				klog.Errorf("failed to delete Load Balancer (Ingress: %s). LoxiLB return response code %d. message: %v", ingress.IP, resp.StatusCode, respBody)
				return err
			}
		}
	}
	return nil
}

func (l *LoxiClient) makeLoxiLoadBalancerModel(externalIP string, port v1.ServicePort, endpointIPs []string) LoadBalancerModel {
	loxiEndpointModelList := []LoadBalancerEndpoint{}
	endpointWeight := int8(LoxiMaxWeight / len(endpointIPs))
	remainderWeight := int8(LoxiMaxWeight % len(endpointIPs))

	for _, endpoint := range endpointIPs {
		weight := endpointWeight
		if remainderWeight > 0 {
			weight++
			remainderWeight--
		}

		loxiEndpointModelList = append(loxiEndpointModelList, LoadBalancerEndpoint{
			EndpointIP: endpoint,
			TargetPort: port.TargetPort.IntVal,
			Weight:     weight,
		})
	}

	return LoadBalancerModel{
		Service: LoadBalancerService{
			ExternalIP: externalIP,
			Port:       port.Port,
			Protocol:   strings.ToLower(string(port.Protocol)),
		},
		Endpoints: loxiEndpointModelList,
	}
}

func (l *LoxiClient) getEndpointsForLB(nodes []*v1.Node) []string {
	var endpoints []string
	for _, node := range nodes {
		addr, err := l.getNodeAddress(node)
		if err != nil {
			klog.Errorf(err.Error())
			continue
		}
		endpoints = append(endpoints, addr)
	}

	return endpoints
}

func (l *LoxiClient) getNodeAddress(node *v1.Node) (string, error) {
	addrs := node.Status.Addresses
	if len(addrs) == 0 {
		return "", errors.New("no address found for host")
	}

	for _, addr := range addrs {
		if addr.Type == v1.NodeInternalIP {
			return addr.Address, nil
		}
	}

	return addrs[0].Address, nil
}
