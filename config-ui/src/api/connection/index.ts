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

import { IConnectionAPI, IConnectionTestResult, IConnectionOldTestResult } from '@/types';
import { request } from '@/utils';

export const list = (plugin: string): Promise<IConnectionAPI[]> => request(`/plugins/${plugin}/connections`);

export const get = (plugin: string, connectionId: ID): Promise<IConnectionAPI> =>
  request(`/plugins/${plugin}/connections/${connectionId}`);

export const create = (plugin: string, payload: Omit<IConnectionAPI, 'id'>): Promise<IConnectionAPI> =>
  request(`/plugins/${plugin}/connections`, { method: 'post', data: payload });

export const remove = (plugin: string, id: ID): Promise<IConnectionAPI> =>
  request(`/plugins/${plugin}/connections/${id}`, { method: 'delete' });

export const update = (plugin: string, id: ID, payload: Omit<IConnectionAPI, 'id'>): Promise<IConnectionAPI> =>
  request(`/plugins/${plugin}/connections/${id}`, {
    method: 'patch',
    data: payload,
  });

export const test = (
  plugin: string,
  connectionId: ID,
  payload?: Partial<
    Pick<
      IConnectionAPI,
      | 'endpoint'
      | 'authMethod'
      | 'username'
      | 'password'
      | 'token'
      | 'appId'
      | 'secretKey'
      | 'proxy'
      | 'rateLimitPerHour'
      | 'dbUrl'
      | 'companyId'
      | 'adminApiKey'
      | 'organization'
      | 'organizationId'
      | 'customHeaders'
    >
  >,
): Promise<IConnectionTestResult> =>
  request(`/plugins/${plugin}/connections/${connectionId}/test`, { method: 'post', data: payload });

export const testOld = (
  plugin: string,
  payload: Pick<
    IConnectionAPI,
    | 'endpoint'
    | 'authMethod'
    | 'username'
    | 'password'
    | 'token'
    | 'appId'
    | 'secretKey'
    | 'proxy'
    | 'rateLimitPerHour'
    | 'dbUrl'
    | 'adminApiKey'
    | 'organization'
    | 'organizationId'
    | 'customHeaders'
  >,
): Promise<IConnectionOldTestResult> => request(`/plugins/${plugin}/test`, { method: 'post', data: payload });
