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

import { IDataScope, IScopeConfig } from '@/types';
import { request } from '@/utils';

export const list = (
  plugin: string,
  connectionId: ID,
  data?: Pagination & {
    blueprints?: boolean;
    searchTerm?: string;
  },
): Promise<{
  count: number;
  scopes: Array<{
    scope: IDataScope;
    scopeConfig?: IScopeConfig;
  }>;
}> =>
  request(`/plugins/${plugin}/connections/${connectionId}/scopes`, {
    data,
  });

export const get = (plugin: string, connectionId: ID, scopeId: ID, payload?: { blueprints: boolean }) =>
  request(`/plugins/${plugin}/connections/${connectionId}/scopes/${encodeURIComponent(scopeId)}`, {
    data: payload,
  });

export const remove = (plugin: string, connectionId: ID, scopeId: ID, onlyData: boolean) =>
  request(`/plugins/${plugin}/connections/${connectionId}/scopes/${encodeURIComponent(scopeId)}?delete_data_only=${onlyData}`, {
    method: 'delete',
  });

export const update = (plugin: string, connectionId: ID, scopeId: ID, payload: any) =>
  request(`/plugins/${plugin}/connections/${connectionId}/scopes/${encodeURIComponent(scopeId)}`, {
    method: 'patch',
    data: payload,
  });

export const batch = (plugin: string, connectionId: ID, payload: any) =>
  request(`/plugins/${plugin}/connections/${connectionId}/scopes`, {
    method: 'put',
    data: payload,
  });

type RemoteQuery = {
  groupId: ID | null;
  pageToken?: string;
};

type RemoteScope = {
  type: 'group' | 'scope';
  parentId: ID | null;
  id: ID;
  name: string;
  fullName: string;
  data: any;
};

export const remote = (
  plugin: string,
  connectionId: ID,
  data: RemoteQuery,
): Promise<{ children: RemoteScope[]; nextPageToken: string }> =>
  request(`/plugins/${plugin}/connections/${connectionId}/remote-scopes`, {
    method: 'get',
    data,
  });

type SearchRemoteQuery = {
  search?: string;
} & Pagination;

export const searchRemote = (
  plugin: string,
  connectionId: ID,
  data: SearchRemoteQuery,
): Promise<{ children: RemoteScope[]; count: number }> =>
  request(`/plugins/${plugin}/connections/${connectionId}/search-remote-scopes`, {
    method: 'get',
    data,
  });

export type ScopeDuplicateConnection = {
  connectionId: ID;
  connectionName: string;
};

// The plugin-specific id field (githubId/gitlabId/bitbucketId) is intentionally
// omitted here since the UI only ever needs `htmlUrl`/`fullName`/`connections`.
export type ScopeDuplicateGroup = {
  htmlUrl: string;
  fullName: string;
  connections: ScopeDuplicateConnection[];
};

// The query param that carries the comma-separated scope ids is named differently per
// plugin (githubIds/gitlabIds/bitbucketIds), so callers pass it in via `idsParam`.
export const scopeDuplicates = (
  plugin: string,
  data?: {
    connectionId?: ID;
    idsParam?: string;
    ids?: string;
  },
): Promise<{ duplicates: ScopeDuplicateGroup[] }> => {
  const { idsParam, ids, ...rest } = data ?? {};
  return request(`/plugins/${plugin}/scope-duplicates`, {
    method: 'get',
    data: idsParam && ids !== undefined ? { ...rest, [idsParam]: ids } : rest,
  });
};
