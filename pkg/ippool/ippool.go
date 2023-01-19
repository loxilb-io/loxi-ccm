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
package ippool

import (
	"net"
	"sync"

	tk "github.com/loxilb-io/loxilib"
)

type IPPool struct {
	IPv4Generator *IPGenerater
	IPv4Pool      *IPSet
	CIDR          string
	IPAlloc       *tk.IPAllocator
	mutex         sync.Mutex
}

// Initailize IP Pool
func NewIPPool(ipa *tk.IPAllocator, netCIDR string) (*IPPool, error) {
	genIPv4, err := InitIPGenerater(netCIDR)
	if err != nil {
		return nil, err
	}

	poolIPv4 := NewSet()
	poolIPv4.Add(genIPv4.GetNetwork().String())
	poolIPv4.Add(genIPv4.GetBroadcastIP().String())

	ipa.AddIPRange(tk.IPClusterDefault, netCIDR)

	return &IPPool{
		CIDR:          netCIDR,
		IPAlloc:       ipa,
		IPv4Generator: genIPv4,
		IPv4Pool:      poolIPv4,
		mutex:         sync.Mutex{},
	}, nil
}

// GetNewIPAddr generate new IP and add key(IP) in IP Pool.
// If IP is already in pool, try to generate next IP.
// Returns nil If all IPs in the subnet are already in the pool.
func (i *IPPool) GetNewIPAddr() net.IP {

	i.mutex.Lock()
	defer i.mutex.Unlock()

	newIP, err := i.IPAlloc.AllocateNewIP(tk.IPClusterDefault, i.CIDR, 0)
	if err != nil {
		return nil
	}

	return newIP
}

// ReturnIPAddr return IPaddress in IP Pool
func (i *IPPool) ReturnIPAddr(IP string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.IPAlloc.DeAllocateIP(tk.IPClusterDefault, i.CIDR, 0, IP)
}

// ReserveIPAddr reserve this IPaddress in IP Pool
func (i *IPPool) ReserveIPAddr(ip string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.IPAlloc.ReserveIP(tk.IPClusterDefault, i.CIDR, 0, ip)
}

// CheckAndReserveIP check and reserve this IPaddress in IP Pool
func (i *IPPool) CheckAndReserveIP(ip string) bool {
	if i.IPv4Generator.CheckIPAddressInSubnet(ip) {
		i.ReserveIPAddr(ip)
		return true
	}

	return false
}
