/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

export interface ICustomHeader {
  key: string;
  value: string;
}

export interface IConnectionAPI {
  id: ID;
  name: string;
  endpoint: string;
  authMethod?: string;
  token?: string;
  adminApiKey?: string;
  username?: string;
  password?: string;
  appId?: string;
  secretKey?: string;
  dbUrl?: string;
  companyId?: number;
  proxy: string;
  rateLimitPerHour?: number;
  organization?: string;
  organizationId?: string;
  customHeaders?: ICustomHeader[];
}

export interface IConnectionTestResult {
  success: boolean;
  message: string;
  causes?: null;
  data?: null;
  tokens?: Array<{
    token: string;
    success: boolean;
    login: string;
    warning: boolean;
  }>;
}

export interface IConnectionOldTestResult {
  success: boolean;
  message: string;
  login?: string;
  installations?: Array<{
    id: number;
    account: {
      login: string;
    };
  }>;
  warning?: string;
}

export enum IConnectionStatus {
  IDLE = 'idle',
  TESTING = 'testing',
  ONLINE = 'online',
  OFFLINE = 'offline',
}

export interface IConnection {
  unique: string;
  plugin: string;
  pluginName: string;
  id: ID;
  name: string;
  status: IConnectionStatus;
  endpoint: string;
  authMethod?: string;
  token?: string;
  adminApiKey?: string;
  username?: string;
  password?: string;
  appId?: string;
  secretKey?: string;
  dbUrl?: string;
  companyId?: number;
  proxy: string;
  rateLimitPerHour?: number;
  organization?: string;
  organizationId?: string;
  customHeaders?: ICustomHeader[];
}
