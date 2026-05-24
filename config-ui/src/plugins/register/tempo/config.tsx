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

import Icon from './assets/icon.png';

export const TempoConfig: IPluginConfig = {
  plugin: 'tempo',
  name: 'Tempo',
  icon: () => <img src={Icon} style={{ width: '100%', height: '100%' }} />,
  sort: 20,
  connection: {
    docLink: 'https://devlake.apache.org/docs/Configuration/Tempo',
    initialValues: {
      endpoint: 'https://api.tempo.io/4',
    },
    fields: [
      'name',
      {
        key: 'endpoint',
        label: 'REST Endpoint',
        subLabel: 'Tempo API v4 base URL',
        placeholder: 'https://api.tempo.io/4',
      },
      'token',
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel: 'Maximum number of API requests per hour. Leave blank for default.',
        defaultValue: 1000,
      },
    ],
  },
  dataScope: {
    title: 'Teams',
    millerColumn: {
      columnCount: 2.5,
    },
  },
  scopeConfig: {
    entities: ['TICKET'],
    transformation: {},
  },
};
