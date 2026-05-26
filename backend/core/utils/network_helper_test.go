/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"testing"
)

func TestCheckDNS(t *testing.T) {
	var hostname = "baidu.com"
	var err = CheckDNS(hostname)
	if err != nil {
		t.Error(err)
	}

	var invalidHostname = "baidu.abc"
	err = CheckDNS(invalidHostname)
	t.Log(err)
	if err == nil {
		t.Errorf("Expected %s, Got nil", "failed")
	}
}

func TestResolvePort(t *testing.T) {
	port, err := ResolvePort("80", "https")
	if err != nil {
		t.Error(err)
	}
	if port != "80" {
		t.Errorf("Expected %s, Got %s", "80", port)
	}
	port, err = ResolvePort("", "http")
	if err != nil {
		t.Error(err)
	}
	if port != "80" {
		t.Errorf("Expected %s, Got %s", "80", port)
	}
	_, err = ResolvePort("", "rabbitmq")
	if err == nil {
		t.Errorf("Expected error %s, Got nil", "schema not found")
	}
	_, err = ResolvePort("", "")
	if err == nil {
		t.Errorf("Expected error %s, Got nil", "you should provide at least one of port or schema")
	}
}
