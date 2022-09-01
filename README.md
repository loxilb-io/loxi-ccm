![build workflow](https://github.com/loxilb-io/loxi-ccm/actions/workflows/docker-publish.yml/badge.svg)

What is loxi-ccm
==========
loxi-ccm is a [CCM][ccmLink] provider that exposes service type external load-balancer implemented by loxilb for kubernetes .
Kubernetes provides the [cloud-provider interface][cloudProviderLink] for the implementation of external cloud provider-specific control logic, and loxi-ccm is an implementation of the cloud-provider interface.

[ccmLink]: https://kubernetes.io/docs/concepts/architecture/cloud-controller/ "k8s Cloud Controller Manager concept"
[cloudProviderLink]: https://github.com/kubernetes/cloud-provider "k8s cloud-provider github page"

Deploy loxi-ccm on kubernetes
===========
The guide below has been tested in environment on Ubuntu 20.04, kubernetes v1.24 (calico CNI)

### 1. Modify k8s ConfigMap
In the manifests/loxi-ccm.yaml manifests file, the ConfigMap is defined as follows
```
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: loxilb-config
  namespace: kube-system
data:
  apiServerURL: "http://192.168.20.54:11111"
  externalIPcidr: 123.123.123.0/24
---
```
The ConfigMap has two values: apiServerURL and externalIPcidr.

- apiServerURL : API Server address of loxilb.
- externalIPcidr : Subnet band to be allocated by loxilb as External IP of the load balancer.

apiServerURL and externalIPcidr must be modified according to the environment of the user using loxi-ccm.

### 2. Deploy loxi-ccm
Once you have modified ConfigMap, you can deploy loxi-ccm using the loxi-ccm.yaml manifest file.
Run the following command on the kubernetes you want to deploy.
```
kubectl apply -f loxi-ccm.yaml
```
After entering the command, check whether loxi-cloud-controller-manager is created in the daemonset of the kube-system namespace.
