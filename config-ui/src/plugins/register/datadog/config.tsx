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

export const DatadogConfig: IPluginConfig = {
  plugin: 'datadog',
  name: 'Datadog',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 21,
  isBeta: true,
  connection: {
    docLink: DOC_URL.PLUGIN.DATADOG.BASIS,
    initialValues: {
      endpoint: 'https://api.datadoghq.com/api/v2/',
    },
    fields: [
      'name',
      {
        key: 'endpoint',
        subLabel: 'The API host for your Datadog site, e.g. https://api.datadoghq.eu/api/v2/ for the EU site.',
        multipleVersions: {
          cloud: 'https://api.datadoghq.com/api/v2/',
          server: '',
        },
      },
      {
        key: 'apiKey',
        label: 'API Key',
        subLabel: 'Organization API key, from Organization Settings > API Keys.',
      },
      {
        key: 'applicationKey',
        label: 'Application Key',
        subLabel: 'Application key with the incident_read scope, from Organization Settings > Application Keys.',
      },
      {
        key: 'webUrl',
        label: 'Datadog UI URL (optional)',
        subLabel:
          'Where your incidents live in the browser, e.g. https://app.datadoghq.com. The API returns no incident link, so without this incidents have no clickable URL.',
      },
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel:
          'By default, DevLake uses 3,600 requests/hour for data collection for Datadog. But you can adjust the collection speed by setting up your desirable rate limit.',
        learnMore: DOC_URL.PLUGIN.DATADOG.RATE_LIMIT,
        defaultValue: 3600,
      },
    ],
  },
  dataScope: {
    title: 'Incident Types',
  },
};
