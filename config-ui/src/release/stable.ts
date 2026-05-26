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

const URLS = {
  TUTORIAL: 'https://devlake.apache.org/docs/Configuration/Tutorial',
  ADVANCED_MODE: {
    EXAMPLES: 'https://devlake.apache.org/docs/Configuration/AdvancedMode/#examples',
  },
  DORA: 'https://devlake.apache.org/docs/DORA/',
  PLUGIN: {
    ASANA: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/Asana',
      TRANSFORMATION: 'https://devlake.apache.org/docs/Configuration/Asana',
    },
    ARGOCD: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/ArgoCD',
      TRANSFORMATION:
        'https://devlake.apache.org/docs/Configuration/ArgoCD#step-3---adding-transformation-rules-optional',
    },
    AZUREDEVOPS: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/AzureDevOps',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/AzureDevOps/#custom-rate-limit-optional',
      AUTH_TOKEN: 'https://devlake.apache.org/docs/Configuration/AzureDevOps#auth-tokens',
      TRANSFORMATION:
        'https://devlake.apache.org/docs/Configuration/AzureDevOps#step-3---adding-transformation-rules-optional',
    },
    AZUREDEVOPS_GO: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/AzureDevOps',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/AzureDevOps/#custom-rate-limit-optional',
      AUTH_TOKEN: 'https://devlake.apache.org/docs/Configuration/AzureDevOps#auth-tokens',
      TRANSFORMATION:
        'https://devlake.apache.org/docs/Configuration/AzureDevOps#step-3---adding-transformation-rules-optional',
    },
    BAMBOO: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/Bamboo',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/Bamboo/#custom-rate-limit-optional',
      TRANSFORMATION:
        'https://devlake.apache.org/docs/Configuration/Bamboo#step-3---adding-transformation-rules-optional',
    },
    BITBUCKET: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/BitBucket',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/BitBucket#fixed-rate-limit-optional',
      API_TOKEN: 'https://devlake.apache.org/docs/Configuration/BitBucket#api-token-recommended',
      APP_PASSWORD: 'https://devlake.apache.org/docs/Configuration/BitBucket#app-password-deprecated',
      TRANSFORMATION:
        'https://devlake.apache.org/docs/Configuration/BitBucket#step-3---adding-transformation-rules-optional',
    },
    BITBUCKET_SERVER: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/BitBucket-Server',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/BitBucket-Server#fixed-rate-limit-optional',
      TRANSFORMATION:
        'https://devlake.apache.org/docs/Configuration/BitBucket-Server#step-3---adding-transformation-rules-optional',
    },
    CIRCLECI: {
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/CircleCI#fixed-rate-limit-optional',
      TRANSFORMATION: 'https://devlake.apache.org/docs/Configuration/CircleCI#step-13---adding-scope-config-optional',
    },
    GITHUB: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/GitHub',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/GitHub#fixed-rate-limit-optional',
      AUTH_TOKEN: 'https://devlake.apache.org/docs/Configuration/GitHub#auth-tokens',
      TRANSFORMATION:
        'https://devlake.apache.org/docs/Configuration/GitHub#step-3---adding-transformation-rules-optional',
    },
    GITLAB: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/GitLab',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/GitLab#fixed-rate-limit-optional',
      AUTH_TOKEN: 'https://devlake.apache.org/docs/Configuration/GitLab#auth-tokens',
      TRANSFORMATION:
        'https://devlake.apache.org/docs/Configuration/GitLab#step-3---adding-transformation-rules-optional',
    },
    JENKINS: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/Jenkins',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/Jenkins#fixed-rate-limit-optional',
      TRANSFORMATION:
        'https://devlake.apache.org/docs/Configuration/Jenkins#step-3---adding-transformation-rules-optional',
    },
    JIRA: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/Jira',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/Jira#fixed-rate-limit-optional',
      API_TOKEN: 'https://devlake.apache.org/docs/Configuration/Jira#api-token',
      PERSONAL_ACCESS_TOKEN: 'https://devlake.apache.org/docs/Configuration/Jira#personal-access-token',
      TRANSFORMATION:
        'https://devlake.apache.org/docs/Configuration/Jira#step-3---adding-transformation-rules-optional',
    },
    OPSGENIE: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/Opsgenie',
      AUTH_TOKEN: 'https://devlake.apache.org/docs/Configuration/Opsgenie#step-11---authentication',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/Opsgenie#fixed-rate-limit-optional',
    },
    PAGERDUTY: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/PagerDuty',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/PagerDuty/#custom-rate-limit-optional',
    },
    ROOTLY: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/Rootly',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/Rootly#fixed-rate-limit-optional',
    },
    SLACK: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/Slack',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/Slack#custom-rate-limit-optional',
    },
    SONARQUBE: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/SonarQube',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/SonarQube/#custom-rate-limit-optional',
    },
    TAPD: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/Tapd',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/Tapd#fixed-rate-limit-optional',
      USERNAMEPASSWORD: 'https://devlake.apache.org/docs/Configuration/Tapd/#usernamepassword',
      TRANSFORMATION:
        'https://devlake.apache.org/docs/Configuration/Tapd#step-3---adding-transformation-rules-optional',
    },
    TEAMBITION: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/Teambition',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/Teambition#ralte-limit-optional',
    },
    WEBHOOK: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/webhook',
    },
    ZENTAO: {
      BASIS: 'https://devlake.apache.org/docs/Configuration/Teambition',
      RATE_LIMIT: 'https://devlake.apache.org/docs/Configuration/Teambition#ralte-limit-optional',
    },
    REFDIFF: 'https://devlake.apache.org/docs/Plugins/refdiff',
  },
  METRICS: {
    BUG_AGE: 'https://devlake.apache.org/docs/Metrics/BugAge',
    MTTR: 'https://devlake.apache.org/docs/Metrics/MTTR',
    BUG_COUNT_PER_1K_LINES_OF_CODE: 'https://devlake.apache.org/docs/Metrics/BugCountPer1kLinesOfCode',
    REQUIREMENT_LEAD_TIME: 'https://devlake.apache.org/docs/Metrics/RequirementLeadTime',
  },
  DATA_MODELS: {
    DEVLAKE_DOMAIN_LAYER_SCHEMA: {
      PULL_REQUEST: 'https://devlake.apache.org/docs/DataModels/DevLakeDomainLayerSchema/#pull-request',
    },
  },
};

export default URLS;
