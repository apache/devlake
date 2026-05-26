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

package auth

import (
	"sync"
	"testing"
)

func TestRevocationCacheEmpty(t *testing.T) {
	c := newRevocationCache()
	if c.IsRevoked("anything") {
		t.Fatal("empty cache should never report revoked")
	}
}

func TestRevocationCacheAdd(t *testing.T) {
	c := newRevocationCache()
	c.Add("jti-1")
	if !c.IsRevoked("jti-1") {
		t.Fatal("expected jti-1 to be revoked")
	}
	if c.IsRevoked("jti-2") {
		t.Fatal("jti-2 was never added; should not be revoked")
	}
}

func TestRevocationCacheConcurrentReadsAndWrites(t *testing.T) {
	c := newRevocationCache()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			c.Add(jtiKey(i))
		}(i)
		go func(i int) {
			defer wg.Done()
			_ = c.IsRevoked(jtiKey(i))
		}(i)
	}
	wg.Wait()
	for i := 0; i < 100; i++ {
		if !c.IsRevoked(jtiKey(i)) {
			t.Fatalf("jti %d missing after concurrent adds", i)
		}
	}
}

func jtiKey(i int) string {
	return "jti-" + string(rune('a'+i%26)) + string(rune('0'+(i/26)%10))
}
