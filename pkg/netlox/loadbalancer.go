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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"loxi-ccm/pkg/ippool"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Bgp        bool     `json:"bgp"`
	Sel        EpSelect `json:"sel"`
	Mode       int32    `json:"mode"`
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
)

func (l *LoxiClient) GetLoxiLoadBalancerAPIUrlString(serverURL *url.URL, subResource []string) string {
	p := path.Join(l.LoxiProviderName, l.LoxiVersion, LoxiLoadBalancerResource)
	if len(subResource) > 0 {
		subPath := path.Join(subResource...)
		p = path.Join(p, subPath)
	}

	lbURL := url.URL{
		Scheme: serverURL.Scheme,
		Host:   serverURL.Host,
		Path:   p,
	}

	return lbURL.String()
}

func (l *LoxiClient) UpdateQueryToUrl(urlStr string, query map[string]string) (string, error) {
	url, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	q := url.Query()
	for key, value := range query {
		q.Add(key, value)
	}

	url.RawQuery = q.Encode()
	return url.String(), nil
}

// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *LoxiClient) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	var resp *http.Response
	var err error

	subResource := []string{
		"all",
	}
	var loxiGetLoadBalancerURLs []string
	for _, u := range l.APIServerURL {
		loxiGetLoadBalancerURLs = append(loxiGetLoadBalancerURLs, l.GetLoxiLoadBalancerAPIUrlString(u, subResource))
	}

	ingresses := service.Status.LoadBalancer.Ingress
	for _, loxiGetLoadBalancerURL := range loxiGetLoadBalancerURLs {
		resp, err = l.RESTClient.GET(ctx, loxiGetLoadBalancerURL)
		if err != nil {
			continue
		}

		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			klog.Errorf("failed to read loxilb(%s) response body in LoadBalancer.GetLoadBalancer(). err: %s", loxiGetLoadBalancerURL, err.Error())
			continue
		}
		if resp.StatusCode != http.StatusOK {
			klog.Errorf("GetLoadBalancer: loxilb(%s) return response status: %d, msg: %v", resp.StatusCode, respBody)

			continue
		}

		lbListModel := LoadBalancerListModel{}
		if err := json.Unmarshal(respBody, &lbListModel); err != nil {
			klog.Errorf("failed to unmarshal response body in LoadBalancer.GetLoadBalancer()")
			return nil, false, err
		}

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
	}

	klog.Infof("not found Load Balancer (Ingresses: %v)", ingresses)
	return nil, false, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *LoxiClient) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	klog.Infof("LoadBalancer.GetLoadBalancerName() returned v1.service.Name: %s", service.Name)
	return service.Name
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *LoxiClient) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	klog.Infof("LoadBalancer.EnsureLoadBalancer() called. service: %s", service.Name)
	if !l.isNeedManage(*service) {
		klog.Infof("service %s is set Spec.LoadBalancerClass %s. ignore.", service.Name, *service.Spec.LoadBalancerClass)
		return nil, nil
	}

	endpointIPs := l.getEndpointsForLB(nodes)

	// validation check if service have ingress IP already
	newIPs := ippool.NewSet()
	updateIngressIPs, err := l.getServiceUpdatedIngressIPs(newIPs, service)
	if err != nil {
		return nil, err
	}

	status := &v1.LoadBalancerStatus{}

	// set defer for deallocate IP when get error
	isFailed := false
	defer func() {
		if isFailed {
			klog.Infof("deallocateOnFailure defer function called")
			for _, ip := range newIPs.GetAll() {
				klog.Infof("ip %s is newIP so retrieve pool", ip)
				l.ExternalIPPool.RetrieveIPv4(ip)
			}
		}
	}()

	var loxiCreateLoadBalancerURLs []string
	for _, u := range l.APIServerURL {
		loxiCreateLoadBalancerURLs = append(loxiCreateLoadBalancerURLs, l.GetLoxiLoadBalancerAPIUrlString(u, nil))
	}
	for _, updateIngressIP := range updateIngressIPs {
		var errChList []chan error

		for _, loxiCreateLoadBalancerURL := range loxiCreateLoadBalancerURLs {
			ch := make(chan error)

			go func(urlStr string, ch chan error) {
				ch <- l.addLoadBalancerRule(ctx, urlStr, updateIngressIP, service, endpointIPs)
			}(loxiCreateLoadBalancerURL, ch)

			errChList = append(errChList, ch)
		}

		isError := true
		for _, errCh := range errChList {
			err := <-errCh
			if err == nil {
				isError = false
			}
		}
		if isError {
			isFailed = isError
			return nil, fmt.Errorf("failed to add loxiLB loadBalancer")
		}

		status.Ingress = append(status.Ingress, v1.LoadBalancerIngress{IP: updateIngressIP})
	}

	return status, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *LoxiClient) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.Infof("LoadBalancer.UpdateLoadBalancer() called by service %s", service.Name)
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
	if !l.isNeedManage(*service) {
		klog.Infof("service %s is set Spec.LoadBalancerClass %s. ignore.", service.Name, *service.Spec.LoadBalancerClass)
		return nil
	}

	ingresses := service.Status.LoadBalancer.Ingress
	ports := service.Spec.Ports
	for _, ingress := range ingresses {
		for _, port := range ports {
			// TODO: need to change argument?
			var loxiDeleteLoadBalancerURLs []string
			for _, u := range l.APIServerURL {
				loxiDeleteLoadBalancerURLs = append(loxiDeleteLoadBalancerURLs, l.GetLoxiLoadBalancerAPIUrlString(u, []string{
					"externalipaddress", ingress.IP,
					"port", strconv.Itoa(int(port.Port)),
					"protocol", strings.ToLower(string(port.Protocol)),
				}))
			}

			var errChList []chan error
			for _, loxiDeleteLoadBalancerURL := range loxiDeleteLoadBalancerURLs {
				loxiDeleteLoadBalancerURLAndQuery, err := l.UpdateQueryToUrl(loxiDeleteLoadBalancerURL, map[string]string{
					"bgp": strconv.FormatBool(l.SetBGP),
				})
				if err != nil {
					klog.Errorf("URL (%s) is incorrect. err: %s", loxiDeleteLoadBalancerURL, err.Error())
				}
				klog.Infof("EnsureLoadBalancerDeleted(): loxiDeleteLoadBalancerURLAndQuery: %s", loxiDeleteLoadBalancerURLAndQuery)

				ch := make(chan error)
				go func(urlStr string, ch chan error) {
					resp, err := l.RESTClient.DELETE(ctx, urlStr)
					// TODO:
					if err != nil {
						klog.Errorf("failed to call LoxiLB API. err: %s", err.Error())
						ch <- err
						return
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						respBody, _ := io.ReadAll(resp.Body)
						klog.Errorf("failed to delete Load Balancer (Ingress: %s). LoxiLB %s return response code %d. message: %v", ingress.IP, urlStr, resp.StatusCode, respBody)
						ch <- fmt.Errorf("LoxiLB %s return response code %d. message: %v", urlStr, resp.StatusCode, respBody)
						return
					}
				}(loxiDeleteLoadBalancerURLAndQuery, ch)
			}

			isError := true
			for _, errCh := range errChList {
				err := <-errCh
				if err == nil {
					isError = false
					break
				}
			}
			if isError {
				return fmt.Errorf("failed to delete loxiLB LoadBalancer")
			}
		}
		l.ExternalIPPool.RetrieveIPv4(ingress.IP)
	}
	return nil
}

func (l *LoxiClient) getLoadBalancerServiceIngressIPs(service *v1.Service) []string {
	var ips []string
	for _, ingress := range service.Status.LoadBalancer.Ingress {
		ips = append(ips, ingress.IP)
	}

	return ips
}

func (l *LoxiClient) makeLoxiLoadBalancerModel(externalIP string, port v1.ServicePort, endpointIPs []string) LoadBalancerModel {
	loxiEndpointModelList := []LoadBalancerEndpoint{}

	if len(endpointIPs) > 0 {
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
				TargetPort: port.NodePort,
				Weight:     weight,
			})
		}
	}

	return LoadBalancerModel{
		Service: LoadBalancerService{
			ExternalIP: externalIP,
			Port:       port.Port,
			Protocol:   strings.ToLower(string(port.Protocol)),
			Bgp:        l.SetBGP,
			Mode:       l.SetLBMode,
		},
		Endpoints: loxiEndpointModelList,
	}
}

func (l *LoxiClient) addLoadBalancerRule(ctx context.Context, lbUrl string, updateIngressIP string, service *v1.Service, endpointIPs []string) error {
	for _, port := range service.Spec.Ports {
		lbModel := l.makeLoxiLoadBalancerModel(updateIngressIP, port, endpointIPs)
		body, err := json.Marshal(lbModel)
		if err != nil {
			klog.Errorf("failed to EnsureLoadBalancer(). err: %s", err.Error())
			return err
		}

		resp, err := l.RESTClient.POST(ctx, lbUrl, body)
		if err != nil {
			klog.Errorf("failed to addLoadBalancerRule() call to LoxiLB(%s) API. err: %s", lbUrl, err.Error())
			return err
		}

		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			klog.Errorf("failed to addLoadBalancerRule(): loxilb API(%s) response status %d.", lbUrl, resp.StatusCode)
			return fmt.Errorf("loxilb(%s) response status %d", resp.Request.URL.String(), resp.StatusCode)
		}
	}

	return nil
}

func (l *LoxiClient) getEndpointsForLB(nodes []*v1.Node) []string {
	var endpoints []string
	for _, node := range nodes {
		addr, err := l.getNodeAddress(*node)
		if err != nil {
			klog.Errorf(err.Error())
			continue
		}
		endpoints = append(endpoints, addr)
	}

	return endpoints
}

func (l *LoxiClient) getNodeAddress(node v1.Node) (string, error) {
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

func (l *LoxiClient) reinstallLoxiLBRules(stopCh <-chan struct{}, aliveCh <-chan string) {
loop:
	for {
		select {
		case <-stopCh:
			break loop
		case aliveUrl := <-aliveCh:
			isSuccess := false
			for retry := 0; retry < 5; retry++ {
				klog.Infof("try reinstall LB rule...")
				if err := l.tryReinstallLoxiLBRules(aliveUrl); err == nil {
					isSuccess = true
					break
				} else {
					time.Sleep(1 * time.Second)
				}
			}
			if !isSuccess {
				klog.Exit("restart loxi-ccm")
			}
		}
	}
}

func (l *LoxiClient) tryReinstallLoxiLBRules(apiUrlStr string) error {
	klog.Infof("LoxiLB alive again so reinstall all LB rules")

	services, err := l.k8sClient.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("failed to get k8s service list when reinstall LB. err: %v", err)
		return err
	}
	nodes, err := l.k8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "!node.kubernetes.io/exclude-from-external-load-balancers"})
	if err != nil {
		klog.Errorf("failed to get k8s nodes list when reinstall LB. err: %v", err)
		return err
	}

	var endpointIPs []string
	for _, node := range nodes.Items {
		nodeIP, err := l.getNodeAddress(node)
		if err != nil {
			klog.Errorf("reinstallLoxiLBRules: failed to get nodeAddress of %s.", node.Name)
			continue
		}
		endpointIPs = append(endpointIPs, nodeIP)
	}

	apiUrl, _ := url.Parse(apiUrlStr)
	lbUrl := l.GetLoxiLoadBalancerAPIUrlString(apiUrl, nil)
	for _, svc := range services.Items {
		if svc.Spec.Type != v1.ServiceTypeLoadBalancer {
			continue
		}
		if !l.isNeedManage(svc) {
			continue
		}
		ingressIPs := l.getLoadBalancerServiceIngressIPs(&svc)

		for _, updateIngressIP := range ingressIPs {
			if err := l.addLoadBalancerRule(context.TODO(), lbUrl, updateIngressIP, &svc, endpointIPs); err != nil {
				return err
			}
		}
	}

	return nil
}

// getServiceUpdatedIngressIPs check validation if service have ingress IP already.
// If service have no ingress IP, assign new IP in IP pool
func (l *LoxiClient) getServiceUpdatedIngressIPs(pool *ippool.IPSet, service *v1.Service) ([]string, error) {
	var updateIngressIPs []string
	newIPs := ippool.NewSet()
	ingressIPs := l.getLoadBalancerServiceIngressIPs(service)
	if len(ingressIPs) >= 1 {
		for _, ingress := range ingressIPs {
			if l.ExternalIPPool.CheckSubnetAndUpdateIPPool(ingress) {
				updateIngressIPs = append(updateIngressIPs, ingress)
			} else {
				newIP := l.ExternalIPPool.AssignNewIPv4()
				if newIP == nil {
					klog.Errorf("failed to generate external IP. IP Pool is full")
					return nil, errors.New("failed to generate external IP. IP Pool is full")
				}
				updateIngressIPs = append(updateIngressIPs, newIP.String())
				newIPs.Add(newIP.String())
			}
		}
	} else {
		newIP := l.ExternalIPPool.AssignNewIPv4()
		if newIP == nil {
			klog.Errorf("failed to generate external IP. IP Pool is full")
			return nil, errors.New("failed to generate external IP. IP Pool is full")
		}
		updateIngressIPs = append(updateIngressIPs, newIP.String())
		newIPs.Add(newIP.String())
	}

	return updateIngressIPs, nil
}

func (l *LoxiClient) isNeedManage(service v1.Service) bool {
	return service.Spec.LoadBalancerClass == nil
}
