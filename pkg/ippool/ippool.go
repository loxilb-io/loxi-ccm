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
package ippool

import (
	"net"
	"sync"
)

type IPPool struct {
	IPv4Generator *IPGenerater
	IPv4Pool      *IPSet
	mutex         sync.Mutex
}

func NewIPPool(netCIDR string) (*IPPool, error) {
	genIPv4, err := InitIPGenerater(netCIDR)
	if err != nil {
		return nil, err
	}

	poolIPv4 := NewSet()
	poolIPv4.Add(genIPv4.GetNetwork().String())
	poolIPv4.Add(genIPv4.GetBroadcastIP().String())

	return &IPPool{
		IPv4Generator: genIPv4,
		IPv4Pool:      poolIPv4,
		mutex:         sync.Mutex{},
	}, nil
}

func (d *IPPool) AssignNewIPv4() net.IP {
	startNewIP := d.IPv4Generator.NextIP()

	d.mutex.Lock()
	defer d.mutex.Unlock()

	id := startNewIP.String()

	if ok := d.IPv4Pool.Contains(id); !ok {
		d.IPv4Pool.Add(id)
		return startNewIP
	}

	for {
		newIP := d.IPv4Generator.NextIP()
		id := newIP.String()
		if ok := d.IPv4Pool.Contains(id); !ok {
			d.IPv4Pool.Add(id)
			return newIP
		}

		if startNewIP.Equal(newIP) {
			return nil
		}
	}
}
