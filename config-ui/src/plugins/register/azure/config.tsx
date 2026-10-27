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

import { ExternalLink } from '@/components';
import { DOC_URL } from '@/release';
import { IPluginConfig } from '@/types';

import Icon from './assets/icon.svg?react';
import { BaseURL, ConnectionOrganization } from './connection-fields';

export const AzureConfig: IPluginConfig = {
  plugin: 'azuredevops',
  name: 'Azure DevOps',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 2,
  connection: {
    docLink: DOC_URL.PLUGIN.AZUREDEVOPS.BASIS,
    initialValues: {},
    fields: [
      'name',
      ({ values, setValues }: any) => (
        <BaseURL key="base-url" value={values.endpoint} onChange={(val) => setValues({ endpoint: val })} />
      ),
      {
        key: 'token',
        label: 'Personal Access Token',
        subLabel: (
          <span>
            <ExternalLink link={DOC_URL.PLUGIN.AZUREDEVOPS.AUTH_TOKEN}>Learn about how to create a PAT</ExternalLink>{' '}
            Please select ALL ACCESSIBLE ORGANIZATIONS for the Organization field when you create the PAT.
          </span>
        ),
      },
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel:
          'By default, DevLake uses 18,000 requests/hour for data collection for Azure DevOps. But you can adjust the collection speed by setting up your desirable rate limit.',
        learnMore: DOC_URL.PLUGIN.AZUREDEVOPS.RATE_LIMIT,
        externalInfo: 'Azure DevOps does not specify a maximum value of rate limit.',
        defaultValue: 18000,
      },
    ],
  },
  dataScope: {
    localSearch: true,
    title: 'Repositories',
    millerColumn: {
      columnCount: 2,
    },
  },
  scopeConfig: {
    entities: ['CODE', 'CODEREVIEW', 'CROSS', 'CICD'],
    transformation: {
      deploymentPattern: '(deploy|push-image)',
      productionPattern: 'prod(.*)',
      refdiff: {
        tagsLimit: 10,
        tagsPattern: '/v\\d+\\.\\d+(\\.\\d+(-rc)*\\d*)*$/',
      },
    },
  },
};

export const AzureGoConfig: IPluginConfig = {
  plugin: 'azuredevops_go',
  name: 'Azure DevOps Go',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 2,
  isBeta: true,
  connection: {
    docLink: DOC_URL.PLUGIN.AZUREDEVOPS.BASIS,
    initialValues: {},
    fields: [
      'name',
      ({ values, setValues }: any) => (
        <BaseURL key="base-url" value={values.endpoint} onChange={(val) => setValues({ endpoint: val })} />
      ),
      {
        key: 'username',
        label: 'Username (Optional for On-Premises)',
        subLabel:
          'For On-Premises Azure DevOps Server, enter your Windows/Domain username if required (e.g. Administrator or domain\\user). Leave empty for Cloud.',
      },
      {
        key: 'token',
        label: 'Personal Access Token',
        subLabel: (
          <span>
            For Azure DevOps Cloud, use a Personal Access Token (PAT). For On-Premises Server, use a PAT or your domain
            password.
          </span>
        ),
      },
      ({ initialValues, values, setValues }: any) => (
        <ConnectionOrganization
          initialValue={initialValues}
          label="Personal Access Token Scope"
          key="ado-organization"
          value={values.organization}
          setValue={(value) => setValues({ organization: value })}
        />
      ),
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel:
          'By default, DevLake uses 18,000 requests/hour for data collection for Azure DevOps. But you can adjust the collection speed by setting up your desirable rate limit.',
        learnMore: DOC_URL.PLUGIN.AZUREDEVOPS.RATE_LIMIT,
        externalInfo: 'Azure DevOps does not specify a maximum value of rate limit.',
        defaultValue: 18000,
      },
    ],
  },
  dataScope: {
    localSearch: true,
    title: 'Repositories',
    millerColumn: {
      columnCount: 2,
    },
  },
  scopeConfig: {
    entities: ['CODE', 'CODEREVIEW', 'CROSS', 'CICD'],
    transformation: {
      deploymentPattern: '(deploy|push-image)',
      productionPattern: 'prod(.*)',
      refdiff: {
        tagsLimit: 10,
        tagsPattern: '/v\\d+\\.\\d+(\\.\\d+(-rc)*\\d*)*$/',
      },
    },
  },
};
