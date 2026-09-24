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

import Icon from './assets/icon.svg';

export const GrafanaIrmConfig: IPluginConfig = {
  plugin: 'grafana_irm',
  name: 'Grafana IRM',
  icon: () => <img src={Icon} style={{ width: '100%', height: '100%' }} />,
  sort: 21,
  isBeta: true,
  connection: {
    docLink: 'https://grafana.com/docs/grafana-cloud/alerting-and-irm/irm/reference/incident-api/get-started/',
    initialValues: {},
    fields: [
      'name',
      {
        key: 'endpoint',
        label: 'Grafana Cloud Stack URL',
        subLabel: 'The base URL of your Grafana Cloud stack, e.g. https://mystack.grafana.net/',
        placeholder: 'https://mystack.grafana.net/',
      },
      {
        key: 'token',
        label: 'Service Account Token',
        subLabel: 'A Grafana Cloud service account token with access to the IRM app.',
      },
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel:
          'By default, DevLake uses 3,600 requests/hour for data collection for Grafana IRM. But you can adjust the collection speed by setting up your desirable rate limit.',
        defaultValue: 3600,
      },
    ],
  },
  dataScope: {
    title: 'Incidents',
  },
};
