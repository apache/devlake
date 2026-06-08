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

package api

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/log"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/helpers/apikeyhelper"
	"github.com/apache/incubator-devlake/server/api/shared"
	"github.com/gin-gonic/gin"
)

const (
	forwardedUserHeader       = "X-Forwarded-User"
	forwardedEmailHeader      = "X-Forwarded-Email"
	forwardedUserSecretHeader = "X-Forwarded-User-Secret"
)

func getOAuthUserInfo(c *gin.Context, forwardedUserSecret string) (*common.User, error) {
	if c == nil {
		return nil, errors.Default.New("request is nil")
	}
	user := strings.TrimSpace(c.GetHeader(forwardedUserHeader))
	if user == "" {
		return nil, nil
	}
	if forwardedUserSecret == "" {
		return nil, errors.Default.New("ignoring forwarded user headers because FORWARDED_USER_SECRET is not configured")
	}
	providedSecret := strings.TrimSpace(c.GetHeader(forwardedUserSecretHeader))
	if subtle.ConstantTimeCompare([]byte(providedSecret), []byte(forwardedUserSecret)) != 1 {
		return nil, errors.Default.New("ignoring forwarded user headers because X-Forwarded-User-Secret did not match")
	}
	email := strings.TrimSpace(c.GetHeader(forwardedEmailHeader))
	return &common.User{
		Name:  user,
		Email: email,
	}, nil
}

func getBasicAuthUserInfo(c *gin.Context, basicRes context.BasicRes) (*common.User, error) {
	if c == nil {
		return nil, errors.Default.New("request is nil")
	}
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		basicRes.GetLogger().Debug("Authorization is empty")
		return nil, nil
	}
	basicAuth := strings.TrimPrefix(authHeader, "Basic ")
	if basicAuth == authHeader || basicAuth == "" {
		return nil, errors.Default.New("invalid basic auth")
	}
	userInfoData, err := base64.StdEncoding.DecodeString(basicAuth)
	if err != nil {
		return nil, errors.Default.Wrap(err, "base64 decode")
	}
	userInfo := strings.Split(string(userInfoData), ":")
	if len(userInfo) != 2 {
		return nil, errors.Default.New("invalid user info data")
	}
	return &common.User{
		Name: userInfo[0],
	}, nil
}

func OAuth2ProxyAuthentication(basicRes context.BasicRes) gin.HandlerFunc {
	logger := basicRes.GetLogger()
	forwardedUserSecret := strings.TrimSpace(basicRes.GetConfigReader().GetString("FORWARDED_USER_SECRET"))
	return func(c *gin.Context) {
		_, exist := c.Get(common.USER)
		if !exist {
			user, err := getOAuthUserInfo(c, forwardedUserSecret)
			if err != nil {
				logger.Warn(err, "rejected forwarded user headers")
			}
			if user == nil || user.Name == "" {
				// fetch with basic auth header
				user, err = getBasicAuthUserInfo(c, basicRes)
				if err != nil {
					logger.Debug("getBasicAuthUserInfo")
				}
			}
			if user != nil && user.Name != "" {
				c.Set(common.USER, user)
			}
		}
		c.Next()
	}
}

type apiBody struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func RestAuthentication(router *gin.Engine, basicRes context.BasicRes) gin.HandlerFunc {

	db := basicRes.GetDal()
	logger := basicRes.GetLogger()
	if db == nil {
		panic(fmt.Errorf("db is not initialised"))
	}
	apiKeyHelper := apikeyhelper.NewApiKeyHelper(basicRes, logger)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// Only open api needs to check api key
		if !strings.HasPrefix(path, "/rest") {
			c.Next()
			return
		}
		path = strings.TrimPrefix(path, "/rest")
		authHeader := c.GetHeader("Authorization")
		ok := CheckAuthorizationHeader(c, logger, db, apiKeyHelper, authHeader, path)
		if !ok {
			c.Abort()
			return
		} else {
			router.HandleContext(c)
			c.Abort()
			return
		}
	}
}

func RequirePushAuthentication(basicRes context.BasicRes) gin.HandlerFunc {
	logger := basicRes.GetLogger()
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/push/") {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Abort()
			c.JSON(http.StatusUnauthorized, &apiBody{
				Success: false,
				Message: "token is missing",
			})
			return
		}
		apiKeyStr := strings.TrimPrefix(authHeader, "Bearer ")
		if apiKeyStr == authHeader || apiKeyStr == "" {
			c.Abort()
			c.JSON(http.StatusUnauthorized, &apiBody{
				Success: false,
				Message: "token is not present or malformed",
			})
			return
		}

		db := basicRes.GetDal()
		if db == nil {
			logger.Error(nil, "db is not initialised")
			c.Abort()
			c.JSON(http.StatusInternalServerError, &apiBody{
				Success: false,
				Message: "database is not initialised",
			})
			return
		}

		apiKeyHelper := apikeyhelper.NewApiKeyHelper(basicRes, logger)
		if !CheckAuthorizationHeader(c, logger, db, apiKeyHelper, authHeader, path) {
			return
		}
		c.Next()
	}
}

func CheckAuthorizationHeader(c *gin.Context, logger log.Logger, db dal.Dal, apiKeyHelper *apikeyhelper.ApiKeyHelper, authHeader, path string) bool {
	if authHeader == "" {
		c.Abort()
		c.JSON(http.StatusUnauthorized, &apiBody{
			Success: false,
			Message: "token is missing",
		})
		return false
	}
	apiKeyStr := strings.TrimPrefix(authHeader, "Bearer ")
	if apiKeyStr == authHeader || apiKeyStr == "" {
		c.Abort()
		c.JSON(http.StatusUnauthorized, &apiBody{
			Success: false,
			Message: "token is not present or malformed",
		})
		return false
	}

	hashedApiKey, err := apiKeyHelper.DigestToken(apiKeyStr)
	if err != nil {
		logger.Error(err, "DigestToken")
		c.Abort()
		c.JSON(http.StatusInternalServerError, &apiBody{
			Success: false,
			Message: err.Error(),
		})
		return false
	}

	apiKey, err := apiKeyHelper.GetApiKey(nil, dal.Where("api_key = ?", hashedApiKey))
	if err != nil {
		c.Abort()
		if db.IsErrorNotFound(err) {
			c.JSON(http.StatusForbidden, &apiBody{
				Success: false,
				Message: "api key is invalid",
			})
		} else {
			logger.Error(err, "query api key from db")
			c.JSON(http.StatusInternalServerError, &apiBody{
				Success: false,
				Message: err.Error(),
			})
		}
		return false
	}

	if apiKey.ExpiredAt != nil && time.Until(*apiKey.ExpiredAt) < 0 {
		c.Abort()
		c.JSON(http.StatusForbidden, &apiBody{
			Success: false,
			Message: "api key has expired",
		})
		return false
	}
	matched, matchErr := regexp.MatchString(apiKey.AllowedPath, path)
	if matchErr != nil {
		logger.Error(err, "regexp match path error")
		c.Abort()
		c.JSON(http.StatusInternalServerError, &apiBody{
			Success: false,
			Message: matchErr.Error(),
		})
		return false
	}
	if !matched {
		c.JSON(http.StatusForbidden, &apiBody{
			Success: false,
			Message: "path doesn't match api key's scope",
		})
		return false
	}

	logger.Info("redirect path: %s to: %s", c.Request.URL.Path, path)
	c.Request.URL.Path = path
	shared.SetUserOnRequest(c, &common.User{
		Name:  apiKey.Creator.Creator,
		Email: apiKey.Creator.CreatorEmail,
	})
	return true
}
