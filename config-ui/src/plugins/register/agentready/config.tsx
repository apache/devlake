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
import { GitHubConnectionSelect, SubmissionsRepo, SubmissionsPath, BranchInput } from './connection-fields';

export const AgentReadyConfig: IPluginConfig = {
  plugin: 'agentready',
  name: 'AgentReady',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 16,
  connection: {
    docLink: 'https://github.com/konflux-ci/devlake/tree/main/backend/plugins/agentready#connection-model',
    fields: [
      'name',
      ({ initialValues, values, errors, setValues, setErrors }: any) => (
        <GitHubConnectionSelect
          initialValue={initialValues?.githubConnectionId ?? values?.githubConnectionId ?? 0}
          value={values?.githubConnectionId ?? 0}
          error={errors?.githubConnectionId ?? ''}
          setValue={(value: number) => setValues({ githubConnectionId: value })}
          setError={(error: string) => setErrors({ githubConnectionId: error })}
        />
      ),
      ({ initialValues, values, errors, setValues, setErrors }: any) => (
        <SubmissionsRepo
          initialValue={initialValues?.submissionsRepo ?? values?.submissionsRepo ?? ''}
          value={values?.submissionsRepo ?? ''}
          error={errors?.submissionsRepo ?? ''}
          setValue={(value: string) => setValues({ submissionsRepo: value })}
          setError={(error: string) => setErrors({ submissionsRepo: error })}
        />
      ),
      ({ initialValues, values, errors, setValues, setErrors }: any) => (
        <SubmissionsPath
          initialValue={initialValues?.submissionsPath ?? values?.submissionsPath ?? ''}
          value={values?.submissionsPath ?? ''}
          error={errors?.submissionsPath ?? ''}
          setValue={(value: string) => setValues({ submissionsPath: value })}
          setError={(error: string) => setErrors({ submissionsPath: error })}
        />
      ),
      ({ initialValues, values, errors, setValues, setErrors }: any) => (
        <BranchInput
          initialValue={initialValues?.branch ?? values?.branch ?? ''}
          value={values?.branch ?? ''}
          error={errors?.branch ?? ''}
          setValue={(value: string) => setValues({ branch: value })}
          setError={(error: string) => setErrors({ branch: error })}
        />
      ),
    ],
    initialValues: {},
  },
  dataScope: {
    title: 'Repositories',
  },
  scopeConfig: {
    entities: ['CODE'],
    transformation: {},
  },
};
