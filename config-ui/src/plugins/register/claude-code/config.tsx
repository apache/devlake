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
import { Token, Organization, CustomHeaders } from './connection-fields';

export const ClaudeCodeConfig: IPluginConfig = {
  plugin: 'claude_code',
  name: 'Claude Code',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 6.6,
  isBeta: true,
  connection: {
    docLink: 'https://github.com/apache/incubator-devlake/tree/main/backend/plugins/claude_code',
    initialValues: {
      endpoint: 'https://api.anthropic.com',
      organization: '',
      token: '',
      customHeaders: [],
      rateLimitPerHour: 1000,
    },
    fields: [
      'name',
      'endpoint',
      ({ type, initialValues, values, setValues, setErrors }: any) => (
        <Organization
          type={type}
          initialValues={initialValues}
          values={values}
          setValues={setValues}
          setErrors={setErrors}
        />
      ),
      ({ type, initialValues, values, setValues, setErrors }: any) => (
        <Token type={type} initialValues={initialValues} values={values} setValues={setValues} setErrors={setErrors} />
      ),
      ({ type, initialValues, values, setValues, setErrors }: any) => (
        <CustomHeaders
          type={type}
          initialValues={initialValues}
          values={values}
          setValues={setValues}
          setErrors={setErrors}
        />
      ),
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel:
          'By default, DevLake uses 1,000 requests/hour for Claude Code usage collection. Adjust this value to throttle collection speed.',
        defaultValue: 1000,
      },
    ],
  },
  dataScope: {
    title: 'Organizations',
  },
  scopeConfig: {
    entities: ['CLAUDE_CODE'],
    transformation: {},
  },
};
