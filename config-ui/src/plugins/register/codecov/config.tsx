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
import { Organization, Service, Token } from './connection-fields';

export const CodecovConfig: IPluginConfig = {
  plugin: 'codecov',
  name: 'Codecov',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 15,
  connection: {
    docLink: 'https://docs.codecov.com',
    initialValues: {
      endpoint: 'https://api.codecov.io',
      service: 'github',
    },
    fields: [
      'name',
      {
        key: 'endpoint',
        subLabel: 'The Codecov API endpoint URL',
        defaultValue: 'https://api.codecov.io',
      },
      ({ type, initialValues, values, errors, setValues, setErrors }: any) => (
        <Service
          key="service"
          type={type}
          initialValues={initialValues}
          values={values}
          errors={errors}
          setValues={setValues}
          setErrors={setErrors}
        />
      ),
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
      ({ type, initialValues, values, errors, setValues, setErrors }: any) => (
        <Token
          key="token"
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
          'By default, DevLake uses dynamic rate limit for optimized data collection for Codecov. But you can adjust the collection speed by entering a fixed value.',
        defaultValue: 1000,
      },
    ],
  },
  dataScope: {
    title: 'Repositories',
  },
};

