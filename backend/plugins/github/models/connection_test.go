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

package models

import (
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func TestAccessTokenSuccess(t *testing.T) {
	accessToken := &GithubConnection{}
	accessToken.AuthMethod = "AccessToken"
	accessToken.Endpoint = "https://api.github.com/"
	accessToken.Name = "test"
	accessToken.Token = "some_token"
	err := accessToken.ValidateConnection(accessToken, validator.New())
	assert.NoError(t, err)
}

func TestAccessTokenFail(t *testing.T) {
	accessToken := &GithubConnection{}
	accessToken.AuthMethod = "AccessToken"
	accessToken.Endpoint = "https://api.github.com/"
	accessToken.Name = "test"
	accessToken.Token = ""
	err := accessToken.ValidateConnection(accessToken, validator.New())
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Token")
	}
}

func TestAppKeyFail(t *testing.T) {
	appkey := &GithubConnection{}
	appkey.AuthMethod = "AppKey"
	appkey.Endpoint = "https://api.github.com/"
	appkey.Name = "test"
	err := appkey.ValidateConnection(appkey, validator.New())
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "InstallationID")
		assert.Contains(t, err.Error(), "AppId")
		assert.Contains(t, err.Error(), "SecretKey")
		println()
	}
}

func TestGithubConnection_Sanitize(t *testing.T) {
	type fields struct {
		GithubConn GithubConn
	}
	tests := []struct {
		name   string
		fields fields
		want   GithubConnection
	}{
		{
			name: "test-empty",
			fields: fields{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: "",
						},
					},
				},
			},
			want: GithubConnection{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: "",
						},
					},
				},
			},
		},
		{
			name: "test-empty-1",
			fields: fields{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: ",",
						},
					},
				},
			},
			want: GithubConnection{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: "",
						},
					},
				},
			},
		},
		{
			name: "test-1",
			fields: fields{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: "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",
						},
					},
				},
			},
			want: GithubConnection{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: "ghp_wFxrXqjC********************bL2WGpTP",
						},
					},
				},
			},
		},
		{
			name: "test-2",
			fields: fields{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: "ghp_wFxrXqjCiAf91234567tCQX8uombbL2WGpTP,ghp_abcdeqjCiAf91234567tCQX8uombbL2WGpTP",
						},
					},
				},
			},
			want: GithubConnection{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: "ghp_wFxrXqjC********************bL2WGpTP,ghp_abcdeqjC********************bL2WGpTP",
						},
					},
				},
			},
		},
		{
			name: "test-3",
			fields: fields{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: "ghp_wFxrXqjCiAf91234567tCQX8uombbL2WGpTP,ghp_abcdeqjCiAf91234567tCQX8uombbL2WGpTP,",
						},
					},
				},
			},
			want: GithubConnection{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: "ghp_wFxrXqjC********************bL2WGpTP,ghp_abcdeqjC********************bL2WGpTP",
						},
					},
				},
			},
		},
		{
			name: "test-4",
			fields: fields{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: "github_pat_11ABMS6RQ0kA7PzN5AThcs_jN8eJeu0BsMa8BDyC12345P9CI67891a2Pu6abcdefk3OEPAJLYMjkfT4U1,",
						},
					},
				},
			},
			want: GithubConnection{
				GithubConn: GithubConn{
					GithubAccessToken: GithubAccessToken{
						AccessToken: api.AccessToken{
							Token: "github_pat_11ABMS6R******************************************************************MjkfT4U1",
						},
					},
				},
			},
		},
		{
			name: "test-5",
			fields: fields{
				GithubConn: GithubConn{
					GithubAppKey: GithubAppKey{
						AppKey: api.AppKey{
							SecretKey: "-----BEGIN RSA PRIVATE KEY-----\nMIIEpQIBAAKCAQEAuornPO7qIw8xasqbHTrAPO6/D1vNU0k00a/bguoUE3kGMMgp\ni2UOhb3JE5QC8y4mO++6arbqfnPFZVrZtg2W20yUpmsPKF5s2RR02+MwjOvZLYlF\nwdUQBqyjLOeuU0dioUxIQBk5jFKAAN5npe87crinruTH/wlQjYziVkXmlOZbMQ78\nrf/heW6WHwd5bRs/QZw+YY/W+7hVMDWh1/0X8d0b3lAT6VQahUIJRFRDZleWGVXW\nF9LMqGxhJd8N7LXX5Nddwdwdwdwdwdwdwddddddddddddddd7sFfEhyfdVKHlCnM\nwK2L6Jj5IJHO/AfGCuGfMRlhkH3ZhRa9Ii7VEa7EUXvzvxg48J83kv7mIWgSV/me\n6q7JZyxN+Z2+DYMabI2F1QFi0QM7eMAyaH6/Ri4qBPysxHqwXxJsuWPLoPqScGV3\nWj1lNLWSozwLU7VyFOUzs9GzAoGBALntw2jbGiZ9mPUJ8pRXNeP7pyhEdy4iYR6E\nOXKd/6l5ZmyQgju428My7PfqyCWdBkdPffzMHXR4UP8vo71HTqhjCRItNR5wXCfY\nJLj6KfFhhLKjR03cqxDHnRizBT9N6TFebsTqatg/7Dt0fc3a7TUD1/+fuRlvHEwe\n9uH/lm59AoGABhd6dWy1uvEYjTNIE8XxWOlCDLPcCfLqxzaAblVOtQVSVTT2XZUB\n6UGQ13OwqNnkuJLaD6Np1mB+sx2gvukZX9M6rGEuxKdkZleLs171EPOVXPoKA2OU\n0/sqvg4vn7vTy7n0/b2xdkb/HfX6+Q7mJKp+D5sT/3UahsW6dKBeBu8=\n-----END RSA PRIVATE KEY-----\n",
						},
					},
				},
			},
			want: GithubConnection{
				GithubConn: GithubConn{
					GithubAppKey: GithubAppKey{
						AppKey: api.AppKey{
							SecretKey: "-----BEGIN RSA PRIVATE KEY-----\nMIIEpQIBAAKCAQEAuo*******************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************************5sT/3UahsW6dKBeBu8=\n-----END RSA PRIVATE KEY-----\n",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := GithubConnection{
				GithubConn: tt.fields.GithubConn,
			}
			assert.Equalf(t, tt.want, connection.Sanitize(), "Sanitize()")
		})
	}
}

func TestTokenTypeClassification(t *testing.T) {
	conn := &GithubConn{}
	assert.Equal(t, GithubTokenTypeClassical, conn.typeIs("ghp_123"))
	assert.Equal(t, GithubTokenTypeClassical, conn.typeIs("gho_123"))
	assert.Equal(t, GithubTokenTypeClassical, conn.typeIs("ghu_123"))
	assert.Equal(t, GithubTokenTypeClassical, conn.typeIs("ghs_123"))
	assert.Equal(t, GithubTokenTypeClassical, conn.typeIs("ghr_123"))
	assert.Equal(t, GithubTokenTypeFineGrained, conn.typeIs("github_pat_123"))
	assert.Equal(t, GithubTokenTypeUnknown, conn.typeIs("some_other_token"))
}

func TestGithubConnection_GetHash(t *testing.T) {
	tests := []struct {
		name       string
		connection GithubConnection
		want       string
	}{
		{
			name: "GitHub App connection should return empty hash to disable caching",
			connection: GithubConnection{
				GithubConn: GithubConn{
					MultiAuth: api.MultiAuth{
						AuthMethod: AppKey,
					},
				},
			},
			want: "",
		},
		{
			name: "PAT connection should use default hash",
			connection: GithubConnection{
				BaseConnection: api.BaseConnection{
					Model: common.Model{
						ID: 123,
					},
				},
				GithubConn: GithubConn{
					MultiAuth: api.MultiAuth{
						AuthMethod: AccessToken,
					},
				},
			},
			want: "1230001-01-01 00:00:00 +0000 UTC", // ID + zero UpdatedAt
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.connection.GetHash(), "GetHash()")
		})
	}
}
