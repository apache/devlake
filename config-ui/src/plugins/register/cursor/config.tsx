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
import { KeyTier, Token } from './connection-fields';
import { renderCursorConnectionMeta } from './connection-meta';
import {
  formatCursorTestMessage,
  onCursorTestSuccess,
  renderCursorTestSummary,
} from './connection-test-summary';

export const CursorConfig: IPluginConfig = {
  plugin: 'cursor',
  name: 'Cursor',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 6.7,
  isBeta: true,
  connection: {
    docLink: 'https://cursor.com/docs/account/teams',
    initialValues: {
      endpoint: 'https://api.cursor.com',
      token: '',
      rateLimitPerHour: 1200,
    },
    formatTestMessage: formatCursorTestMessage,
    onTestSuccess: onCursorTestSuccess,
    renderTestSummary: renderCursorTestSummary,
    renderConnectionMeta: renderCursorConnectionMeta,
    fields: [
      'name',
      'endpoint',
      ({ type, initialValues, values, setValues, setErrors }: any) => (
        <Token type={type} initialValues={initialValues} values={values} setValues={setValues} setErrors={setErrors} />
      ),
      ({ type, initialValues, values }: any) => (
        <KeyTier type={type} initialValues={initialValues} values={values} />
      ),
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel:
          'By default, DevLake uses 1,200 requests/hour for Cursor Admin API collection (documented limit is 20 requests/minute). Adjust this value to throttle collection speed.',
        defaultValue: 1200,
      },
    ],
  },
  dataScope: {
    title: 'Team',
  },
  scopeConfig: {
    entities: ['CROSS'],
    transformation: {},
  },
};
