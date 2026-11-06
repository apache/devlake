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
import { GlpiDataScope } from './data-scope';

export const GlpiConfig: IPluginConfig = {
  plugin: 'glpi',
  name: 'GLPI',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 19.6,
  isBeta: true,
  connection: {
    docLink: '',
    initialValues: {},
    fields: [
      'name',
      {
        key: 'endpoint',
        subLabel: 'Base URL of your GLPI instance, e.g. https://glpi.example.com',
      },
      {
        key: 'token',
        label: 'Application Token',
        subLabel: 'From GLPI: Setup → General → API → Application token',
      },
      {
        key: 'secretKey',
        label: 'User Token',
        subLabel: 'From GLPI: My settings → Remote access keys → API token',
        placeholder: 'Your GLPI User Token',
      },
      {
        key: 'timezone',
        required: true,
        subLabel:
          'Must match the timezone configured on your GLPI server (PHP date.timezone or Setup → General → Default values). Search by city or UTC offset.',
        placeholder: 'Search timezone…',
      },
      'proxy',
    ],
  },
  dataScope: {
    title: 'GLPI Entities',
    render: ({ connectionId, disabledItems, selectedItems, onChangeSelectedItems }) => (
      <GlpiDataScope
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
      collectAssignees: true,
      collectFollowups: false,
      collectTasks: false,
      collectLogs: false,
      collectSolutions: false,
      collectTicketLinks: false,
    },
  },
};
