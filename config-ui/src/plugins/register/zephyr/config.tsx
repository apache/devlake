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
import { ZephyrDataScope } from './data-scope';

export const ZephyrConfig: IPluginConfig = {
  plugin: 'zephyr',
  name: 'Zephyr',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 17.5,
  isBeta: true,
  connection: {
    docLink:
      'https://support.smartbear.com/zephyr-scale-cloud/docs/en/rest-api/generating-api-access-tokens.html',
    initialValues: {
      endpoint: 'https://api.zephyrscale.smartbear.com/v2',
    },
    fields: [
      'name',
      {
        key: 'endpoint',
        subLabel:
          'Zephyr Scale Cloud API base URL. Default (US): https://api.zephyrscale.smartbear.com/v2. EU: https://eu.api.zephyrscale.smartbear.com/v2',
      },
      {
        key: 'token',
        label: 'API Access Token',
        type: 'password',
        subLabel: 'From Jira: Profile → Zephyr Scale API Access Tokens',
      },
      'proxy',
    ],
  },
  dataScope: {
    title: 'Jira Projects',
    render: ({ connectionId, disabledItems, selectedItems, onChangeSelectedItems }) => (
      <ZephyrDataScope
        connectionId={connectionId}
        disabledItems={disabledItems}
        selectedItems={selectedItems}
        onChangeSelectedItems={onChangeSelectedItems}
      />
    ),
  },
  scopeConfig: {
    entities: ['CODEQUALITY'],
    transformation: {
      testTypeCustomField: 'Test Type',
    },
  },
};
