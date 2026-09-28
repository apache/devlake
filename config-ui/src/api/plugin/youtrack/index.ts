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

import { request } from '@/utils';

export type YoutrackBundleValue = {
  id: string;
  name: string;
  isResolved?: boolean;
  ordinal?: number;
  archived?: boolean;
};

// YoutrackProjectCustomField mirrors one entry of the
// /admin/projects/{id}/customFields response as requested by the widget's
// fields param. `project(shortName)` rides along so the union tables can
// badge projects by the issue-key prefix users recognize (PROJ in PROJ-123).
export type YoutrackProjectCustomField = {
  id: string;
  $type: string;
  field?: {
    id: string;
    name: string;
    fieldType?: {
      id: string;
      valueType: string;
      isMultiValue: boolean;
    };
  };
  bundle?: {
    id: string;
    values?: YoutrackBundleValue[];
  } | null;
  project?: {
    shortName: string;
  } | null;
};

export type YoutrackScopeConfigProjects = {
  count: number;
  projects?: Array<{
    name: string;
    blueprintId: ID;
    scopes?: Array<{
      scopeId: string;
      scopeName: string;
    }>;
  }>;
};

export const customFields = (prefix: string, projectId: string): Promise<YoutrackProjectCustomField[]> =>
  request(`${prefix}/admin/projects/${projectId}/customFields`, {
    data: {
      fields:
        'id,$type,field(id,name,fieldType(id,valueType,isMultiValue)),bundle(id,values(id,name,isResolved,ordinal,archived)),project(shortName)',
    },
  });

// scopeConfigProjects is the singular, un-nested route (Linear's asymmetry):
// it deliberately takes no connectionId.
export const scopeConfigProjects = (scopeConfigId: ID): Promise<YoutrackScopeConfigProjects> =>
  request(`/plugins/youtrack/scope-config/${scopeConfigId}/projects`);
