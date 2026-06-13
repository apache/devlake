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

package shared

import (
	"context"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/gin-gonic/gin"
)

// userContextKey is a dedicated, unexported type for storing the
// authenticated user on the request's context.Context. A dedicated type
// avoids collisions with other packages and satisfies go vet (which flags
// basic types such as plain strings used as context keys).
type userContextKey struct{}

// SetUserOnRequest stores the authenticated user on the request's
// context.Context so the identity survives gin's Engine.HandleContext
// re-dispatch, which calls Context.reset() and wipes gin's Keys (where
// c.Set stores values). Callers that re-dispatch (e.g. the /rest open-API
// path rewrite) must use this so the terminal RequireAuth gate can still
// see the user. It also mirrors the value into gin Keys via c.Set for the
// common, non-re-dispatched case.
func SetUserOnRequest(c *gin.Context, user *common.User) {
	c.Set(common.USER, user)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), userContextKey{}, user))
}

func GetUser(c *gin.Context) (*common.User, bool) {
	if userObj, exist := c.Get(common.USER); exist {
		if user, ok := userObj.(*common.User); ok {
			return user, true
		}
	}
	// Fall back to the request context, which survives Engine.HandleContext
	// re-dispatch (unlike gin Keys, which Context.reset() clears).
	if user, ok := c.Request.Context().Value(userContextKey{}).(*common.User); ok {
		return user, true
	}
	return nil, false
}
