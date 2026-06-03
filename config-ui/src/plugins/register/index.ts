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

import { ArgoCDConfig } from './argocd';
import { AsanaConfig } from './asana';
import { AzureConfig, AzureGoConfig } from './azure';
import { BambooConfig } from './bamboo';
import { BitbucketConfig } from './bitbucket';
import { BitbucketServerConfig } from './bitbucket-server';
import { CircleCIConfig } from './circleci';
import { ClaudeCodeConfig } from './claude-code';
import { GitHubConfig } from './github';
import { GhCopilotConfig } from './gh-copilot';
import { GitLabConfig } from './gitlab';
import { JenkinsConfig } from './jenkins';
import { JiraConfig } from './jira';
import { LinearConfig } from './linear';
import { PagerDutyConfig } from './pagerduty';
import { RootlyConfig } from './rootly';
import { SonarQubeConfig } from './sonarqube';
import { TAPDConfig } from './tapd';
import { WebhookConfig } from './webhook';
import { ZenTaoConfig } from './zentao';
import { OpsgenieConfig } from './opsgenie';
import { QDevConfig } from './q-dev';
import { TeambitionConfig } from './teambition';
import { TestmoConfig } from './testmo';
import { SlackConfig } from './slack/config';

export const pluginConfigs: IPluginConfig[] = [
  ArgoCDConfig,
  AsanaConfig,
  AzureConfig,
  AzureGoConfig,
  BambooConfig,
  BitbucketConfig,
  BitbucketServerConfig,
  CircleCIConfig,
  ClaudeCodeConfig,
  GitHubConfig,
  GhCopilotConfig,
  GitLabConfig,
  JenkinsConfig,
  JiraConfig,
  LinearConfig,
  PagerDutyConfig,
  RootlyConfig,
  SlackConfig,
  QDevConfig,
  SonarQubeConfig,
  TAPDConfig,
  TestmoConfig,
  ZenTaoConfig,
  WebhookConfig,
  OpsgenieConfig,
  TeambitionConfig,
].sort((a, b) => a.sort - b.sort);
