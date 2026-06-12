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

export const RootlyConfig: IPluginConfig = {
  plugin: 'rootly',
  name: 'Rootly',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 16,
  isBeta: true,
  connection: {
    docLink: DOC_URL.PLUGIN.ROOTLY.BASIS,
    initialValues: {
      endpoint: 'https://api.rootly.com/v1/',
    },
    fields: [
      'name',
      {
        key: 'endpoint',
        multipleVersions: {
          cloud: 'https://api.rootly.com/v1/',
          server: '',
        },
      },
      'token',
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel:
          'By default, DevLake uses 3,600 requests/hour for data collection for Rootly. But you can adjust the collection speed by setting up your desirable rate limit.',
        learnMore: DOC_URL.PLUGIN.ROOTLY.RATE_LIMIT,
        defaultValue: 3600,
      },
    ],
  },
  dataScope: {
    title: 'Services',
  },
};
