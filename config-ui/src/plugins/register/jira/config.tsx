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
import { Auth } from './connection-fields';

export const JiraConfig: IPluginConfig = {
  plugin: 'jira',
  name: 'Jira',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 9,
  connection: {
    docLink: DOC_URL.PLUGIN.JIRA.BASIS,
    fields: [
      'name',
      ({ type, initialValues, values, errors, setValues, setErrors }: any) => (
        <Auth
          key="auth"
          type={type}
          initialValues={initialValues}
          values={values}
          errors={errors}
          setValues={setValues}
          setErrors={setErrors}
        />
      ),
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel:
          'By default, DevLake uses dynamic rate limit for optimized data collection for Jira. But you can adjust the collection speed by setting up your desirable rate limit.',
        learnMore: DOC_URL.PLUGIN.JIRA.RATE_LIMIT,
        externalInfo:
          'Jira Cloud does not specify a maximum value of rate limit.  For Jira Server / Jira Data Center, please contact your admin for more information.',
        defaultValue: 10000,
      },
    ],
  },
  dataScope: {
    title: 'Boards',
  },
  scopeConfig: {
    entities: ['TICKET', 'CROSS'],
    transformation: {
      storyPointField: '',
      typeMappings: {},
      remotelinkCommitShaPattern: '',
      remotelinkRepoPattern: [],
    },
  },
};
