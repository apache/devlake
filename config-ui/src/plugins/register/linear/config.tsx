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

export const LinearConfig: IPluginConfig = {
  plugin: 'linear',
  name: 'Linear',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 13,
  connection: {
    docLink: 'https://developers.linear.app/docs',
    initialValues: {
      endpoint: 'https://api.linear.app/graphql',
    },
    fields: [
      'name',
      {
        key: 'endpoint',
        label: 'Endpoint',
        subLabel: 'Linear GraphQL API base URL.',
      },
      {
        key: 'token',
        label: 'API Key',
        subLabel: 'Your Linear personal API key (Settings → Security & access → Personal API keys).',
      },
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel: 'Maximum number of API requests per hour. Leave blank for the default (1500).',
        defaultValue: 1500,
      },
    ],
  },
  dataScope: {
    title: 'Teams',
    searchPlaceholder: 'Search teams...',
  },
};
