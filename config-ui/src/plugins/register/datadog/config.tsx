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
import { ApiKey, ApplicationKey, WebUrl } from './connection-fields';

export const DatadogConfig: IPluginConfig = {
  plugin: 'datadog',
  name: 'Datadog',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 5,
  isBeta: true,
  connection: {
    // No docLink until the Datadog page exists on the DevLake site; the
    // banner is hidden while this is empty.
    docLink: '',
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
      ({ type, initialValues, values, setValues, setErrors }: any) => (
        <ApiKey
          key="apiKey"
          type={type}
          initialValue={initialValues.apiKey ?? ''}
          value={values.apiKey ?? ''}
          setValue={(value) => setValues({ apiKey: value })}
          setError={(value) => setErrors({ apiKey: value })}
        />
      ),
      ({ type, initialValues, values, setValues, setErrors }: any) => (
        <ApplicationKey
          key="applicationKey"
          type={type}
          initialValue={initialValues.applicationKey ?? ''}
          value={values.applicationKey ?? ''}
          setValue={(value) => setValues({ applicationKey: value })}
          setError={(value) => setErrors({ applicationKey: value })}
        />
      ),
      ({ initialValues, values, setValues }: any) => (
        <WebUrl
          key="webUrl"
          initialValue={initialValues.webUrl ?? ''}
          value={values.webUrl ?? ''}
          setValue={(value) => setValues({ webUrl: value })}
        />
      ),
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel:
          'By default, DevLake uses 3,600 requests/hour for data collection for Datadog. But you can adjust the collection speed by setting up your desirable rate limit.',

        defaultValue: 3600,
      },
    ],
  },
  dataScope: {
    title: 'Incident Types',
  },
};
