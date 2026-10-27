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

import { IPluginConfig } from '@/types';

import Icon from './assets/icon.svg?react';
import { RedmineDataScope } from './data-scope';

export const RedmineConfig: IPluginConfig = {
  plugin: 'redmine',
  name: 'Redmine',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 19.5,
  isBeta: true,
  connection: {
    docLink: 'https://www.redmine.org/projects/redmine/wiki/Rest_api',
    initialValues: {},
    fields: [
      'name',
      {
        key: 'endpoint',
        subLabel: 'Base URL of your Redmine instance, e.g. https://redmine.example.com',
      },
      {
        key: 'token',
        label: 'API Key',
        subLabel: 'From Redmine: My account → API access key',
      },
      'proxy',
    ],
  },
  dataScope: {
    title: 'Projects',
    render: ({ connectionId, disabledItems, selectedItems, onChangeSelectedItems }) => (
      <RedmineDataScope
        connectionId={connectionId}
        disabledItems={disabledItems}
        selectedItems={selectedItems}
        onChangeSelectedItems={onChangeSelectedItems}
      />
    ),
  },
  scopeConfig: {
    entities: ['TICKET'],
    transformation: {
      collectJournals: false,
      typeMappings: {},
      statusMappings: {},
    },
  },
};
