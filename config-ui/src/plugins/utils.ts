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

import React from 'react';

import PluginIcon from '@/images/plugin-icon.svg?react';

import { pluginConfigs } from './register';
import { IPluginConfig } from '@/types';

export const getPluginScopeId = (plugin: string, scope: any) => {
  switch (plugin) {
    case 'github':
      return `${scope.githubId}`;
    case 'jira':
      return `${scope.boardId}`;
    case 'gitlab':
      return `${scope.gitlabId}`;
    case 'jenkins':
      return `${scope.fullName}`;
    case 'bitbucket':
      return `${scope.bitbucketId}`;
    case 'bitbucket_server':
      return `${scope.bitbucketId}`;
    case 'sonarqube':
      return `${scope.projectKey}`;
    case 'bamboo':
      return `${scope.planKey}`;
    case 'argocd':
      return `${scope.name}`;
    case 'asana':
      return `${scope.gid}`;
    default:
      return `${scope.id}`;
  }
};

export const getPluginScopeName = (plugin: string, scope: any) => {
  if (!scope) {
    return '';
  }

  if (plugin === 'claude_code') {
    const scopeData = scope.data ?? scope;

    return `${scopeData.organization ?? scope.organization ?? scope.fullName ?? scope.name ?? scope.id ?? ''}`.trim();
  }

  if (plugin === 'gh-copilot') {
    const scopeData = scope.data ?? scope;

    const rawId = `${scope.id ?? scopeData.id ?? scope.fullName ?? scope.name ?? ''}`.trim();
    const enterprise = `${scopeData.enterprise ?? ''}`.trim();
    const organization = `${scopeData.organization ?? ''}`.trim();

    if (enterprise && organization) {
      return `${rawId} (Enterprise + Organization)`;
    }
    if (enterprise) {
      return `${rawId} (Enterprise)`;
    }
    if (organization) {
      return `${rawId} (Organization)`;
    }
    if (rawId.includes('/')) {
      return `${rawId} (Enterprise + Organization)`;
    }
    return rawId;
  }

  return `${scope.fullName ?? scope.name ?? scope.id ?? ''}`;
};

const pluginAliasMap: Record<string, string> = {
  claude_code: 'claude_code',
  copilot: 'gh-copilot',
};

const aliasByTarget = Object.entries(pluginAliasMap).reduce<Record<string, string[]>>((acc, [alias, target]) => {
  if (!acc[target]) {
    acc[target] = [];
  }
  acc[target].push(alias);
  return acc;
}, {});

export const getRegisterPlugins = () => {
  const ordered: string[] = [];
  const seen = new Set<string>();

  for (const config of pluginConfigs) {
    if (!seen.has(config.plugin)) {
      ordered.push(config.plugin);
      seen.add(config.plugin);
    }

    for (const alias of aliasByTarget[config.plugin] ?? []) {
      if (!seen.has(alias)) {
        ordered.push(alias);
        seen.add(alias);
      }
    }
  }

  for (const alias of Object.keys(pluginAliasMap)) {
    if (!seen.has(alias)) {
      ordered.push(alias);
    }
  }

  return ordered;
};

export const getPluginConfig = (name: string): IPluginConfig => {
  let pluginConfig = pluginConfigs.find((it) => it.plugin === name);
  if (!pluginConfig && pluginAliasMap[name]) {
    pluginConfig = pluginConfigs.find((it) => it.plugin === pluginAliasMap[name]);
  }
  if (!pluginConfig) {
    pluginConfig = {
      plugin: name,
      name: name,
      icon: ({ color }) => React.createElement(PluginIcon, { fill: color }),
      sort: 101,
      connection: {
        docLink: '',
        initialValues: {},
        fields: [],
      },
      dataScope: {
        title: '',
      },
    };
  }
  return pluginConfig;
};
