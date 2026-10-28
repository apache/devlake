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
import { 
  QuayOrganization, 
  ProjectSelect, 
  CIToolSelect, 
  GitHubOrganization, 
  GitHubToken,
  JUnitRegex,
} from './connection-fields';

export const TestRegistryConfig: IPluginConfig = {
  plugin: 'testregistry',
  name: 'Test Registry',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 17,  // Between Testmo (17) and ZenTao (18)
  connection: {
    docLink: '',
    fields: [
      'name',
      ({ type, initialValues, values, errors, setValues, setErrors }: any) => (
        <ProjectSelect
          initialValue={initialValues?.project ?? values?.project ?? ''}
          value={values?.project ?? ''}
          error={errors?.project ?? ''}
          setValue={(value: string) => setValues({ project: value })}
          setError={(error: string) => setErrors({ project: error })}
        />
      ),
      ({ type, initialValues, values, errors, setValues, setErrors }: any) => (
        <CIToolSelect
          initialValue={initialValues?.ciTool ?? values?.ciTool ?? ''}
          value={values?.ciTool ?? ''}
          error={errors?.ciTool ?? ''}
          setValue={(value: string) => {
            // Update CI tool and clear conditional fields and their errors synchronously
            setValues({ ciTool: value });

            // Clear conditional fields and their errors based on selected CI tool
            if (value === 'Openshift CI') {
              setValues({ quayOrganization: '' });
              // Clear Quay errors, keep junitRegex as it's common to both
              setErrors({ quayOrganization: '', junitRegex: '' });
            } else if (value === 'Tekton CI') {
              setValues({ githubOrganization: '', githubToken: '' });
              // Clear GitHub errors, keep junitRegex as it's common to both
              setErrors({ githubOrganization: '', githubToken: '', junitRegex: '' });
            } else {
              // No CI tool selected - clear all conditional field errors
              setErrors({ quayOrganization: '', githubOrganization: '', githubToken: '', junitRegex: '' });
            }
          }}
          setError={(error: string) => setErrors({ ciTool: error })}
        />
      ),
      // Conditional fields based on CI tool selection
      ({ type, initialValues, values, errors, setValues, setErrors }: any) => {
        const ciTool = values?.ciTool ?? initialValues?.ciTool ?? '';

        if (ciTool === 'Openshift CI') {
          return (
            <>
              <GitHubOrganization
                initialValue={initialValues?.githubOrganization ?? values?.githubOrganization ?? ''}
                value={values?.githubOrganization ?? ''}
                error={errors?.githubOrganization ?? ''}
                setValue={(value: string) => setValues({ githubOrganization: value })}
                setError={(error: string) => setErrors({ githubOrganization: error })}
              />
              <GitHubToken
                initialValue={initialValues?.githubToken ?? values?.githubToken ?? ''}
                value={values?.githubToken ?? ''}
                error={errors?.githubToken ?? ''}
                setValue={(value: string) => setValues({ githubToken: value })}
                setError={(error: string) => setErrors({ githubToken: error })}
              />
              <JUnitRegex
                initialValue={initialValues?.junitRegex ?? values?.junitRegex ?? ''}
                value={values?.junitRegex ?? ''}
                error={errors?.junitRegex ?? ''}
                setValue={(value: string) => setValues({ junitRegex: value })}
                setError={(error: string) => setErrors({ junitRegex: error })}
              />
            </>
          );
        }

        if (ciTool === 'Tekton CI') {
          return (
            <>
              <QuayOrganization
                initialValue={initialValues?.quayOrganization ?? values?.quayOrganization ?? ''}
                value={values?.quayOrganization ?? ''}
                error={errors?.quayOrganization ?? ''}
                setValue={(value: string) => setValues({ quayOrganization: value })}
                setError={(error: string) => setErrors({ quayOrganization: error })}
              />
              <JUnitRegex
                initialValue={initialValues?.junitRegex ?? values?.junitRegex ?? ''}
                value={values?.junitRegex ?? ''}
                error={errors?.junitRegex ?? ''}
                setValue={(value: string) => setValues({ junitRegex: value })}
                setError={(error: string) => setErrors({ junitRegex: error })}
              />
            </>
          );
        }

        return null;
      },
    ],
    initialValues: {},
  },
  dataScope: {
    title: 'Scopes',
  },
  scopeConfig: {
    entities: ['CICD'],
    transformation: {},
  },
};
