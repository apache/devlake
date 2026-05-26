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

import { DOC_URL } from '@/release';
import { IPluginConfig } from '@/types';

import Icon from './assets/icon.svg?react';

export const AsanaConfig: IPluginConfig = {
  plugin: 'asana',
  name: 'Asana',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 12,
  connection: {
    docLink: DOC_URL.PLUGIN.ASANA.BASIS,
    initialValues: {
      endpoint: 'https://app.asana.com/api/1.0/',
    },
    fields: [
      'name',
      {
        key: 'endpoint',
        label: 'Endpoint',
        subLabel: 'Asana API base URL.',
      },
      'token',
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel: 'Maximum number of API requests per hour. Leave blank for default.',
        defaultValue: 150,
      },
    ],
  },
  dataScope: {
    title: 'Projects',
    millerColumn: {
      columnCount: 4,
      firstColumnTitle: 'Workspaces',
    },
    searchPlaceholder: 'Search projects...',
  },
  scopeConfig: {
    entities: ['TICKET'],
    transformation: {
      issueTypeRequirement: '(feat|feature|story|requirement)',
      issueTypeBug: '(bug|defect|broken)',
      issueTypeIncident: '(incident|outage|failure)',
    },
  },
};
