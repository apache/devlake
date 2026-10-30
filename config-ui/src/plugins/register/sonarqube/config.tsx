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

import { Organization } from './connection-fields/organization';

export const SonarQubeConfig: IPluginConfig = {
  plugin: 'sonarqube',
  name: 'SonarQube',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 14,
  connection: {
    docLink: DOC_URL.PLUGIN.SONARQUBE.BASIS,
    initialValues: {
      endpoint: 'https://sonarcloud.io/api/',
    },
    fields: [
      'name',
      {
        key: 'endpoint',
        multipleVersions: {
          cloud: 'https://sonarcloud.io/api/',
          server: ' ',
        },
        cloudName: 'SonarCloud',
        subLabel: 'The URL should be `http://<sonarqube-host>:<port>/api/`',
      },
      ({ type, initialValues, values, errors, setValues, setErrors }: any) => (
        <Organization
          key="organization"
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
        subLabel:
          'A token with Browse permission on the projects you want is sufficient for listing projects and collecting issues, hotspots, and file metrics. Listing Accounts (users) may require a system admin token on some SonarQube instances.',
      },
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel:
          'By default, DevLake uses 10,000 requests/hour for data collection for SonarQube. But you can adjust the collection speed by setting up your desirable rate limit.',
        learnMore: DOC_URL.PLUGIN.SONARQUBE.RATE_LIMIT,
        externalInfo: 'SonarQube does not specify a maximum value of rate limit.',
        defaultValue: 10000,
      },
    ],
  },
  dataScope: {
    title: 'Repositories',
    searchSelectAll: true,
  },
};
