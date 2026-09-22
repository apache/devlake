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
import { Endpoint } from './connection-fields';

export const YoutrackConfig: IPluginConfig = {
  plugin: 'youtrack',
  name: 'YouTrack',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 21,
  connection: {
    docLink: 'https://www.jetbrains.com/help/youtrack/devportal/youtrack-rest-api.html',
    initialValues: {},
    // the test API's diagnostics are the point of the test button: the
    // success message names the token's login (and warns on a non-perm:
    // token); failures (guest fallback, missing /youtrack prefix) already
    // surface through the error toast
    showTestResultMessage: true,
    fields: [
      'name',
      ({ type, initialValues, values, errors, setValues, setErrors }: any) => (
        <Endpoint
          key="endpoint"
          type={type}
          initialValues={initialValues}
          values={values}
          errors={errors}
          setValues={setValues}
          setErrors={setErrors}
        />
      ),
      {
        key: 'token',
        label: 'Permanent Token',
        subLabel:
          'Your YouTrack permanent token — it starts with `perm:` (YouTrack → Profile → Account Security → New Token). The Test button detects a token that silently authenticated as guest.',
      },
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel: 'Maximum number of API requests per hour. Leave blank for the default (10000).',
        defaultValue: 10000,
      },
    ],
  },
  dataScope: {
    // the backend serves remote-scopes only (no search-remote-scopes), so
    // search is client-side over the loaded projects (jenkins pattern)
    localSearch: true,
    title: 'Projects',
  },
  scopeConfig: {
    entities: ['TICKET', 'CROSS'],
    transformation: {
      typeField: '',
      stateField: '',
      priorityField: '',
      assigneeField: '',
      storyPointField: '',
      dueDateField: '',
      typeMappings: {},
      statusMappings: {},
    },
  },
};
