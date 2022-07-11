/*
Copyright © 2022 Netlox Inc. <inhogog2@netlox.io>

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

type IPSet struct {
	ipaddress map[string]struct{}
}

var exists = struct{}{}

func NewSet() *IPSet {
	s := &IPSet{}
	s.ipaddress = make(map[string]struct{})
	return s
}

func (s *IPSet) Add(value string) {
	s.ipaddress[value] = exists
}

func (s *IPSet) Remove(value string) {
	delete(s.ipaddress, value)
}

func (s *IPSet) Contains(value string) bool {
	_, c := s.ipaddress[value]
	return c
}

func (s *IPSet) IsEqual(Pair *IPSet) (isEqual bool, added, deleted IPSet) {
	// Set_A == Set_B 라는것은
	// Set_A 가 Set_B 의 부분집합인 동시에
	// Set_B 가 Set_A 의 부분집합인 경우를 말한다.
	isEqual = true
	for values := range s.ipaddress {
		c := Pair.Contains(values)
		if !c {
			isEqual = false
			deleted.Add(values)
		}
	}

	for values := range Pair.ipaddress {
		c := s.Contains(values)
		if !c {
			isEqual = false
			added.Add(values)
		}
	}

	return isEqual, added, deleted
}
