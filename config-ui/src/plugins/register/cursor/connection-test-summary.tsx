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

import { Alert } from 'antd';

import { IConnectionOldTestResult, IConnectionTestResult } from '@/types';

import { CURSOR_TIER_LABELS } from './connection-meta';

type CursorTestResult = IConnectionTestResult | IConnectionOldTestResult;

const tierLabel = CURSOR_TIER_LABELS;

const capabilityLines = (result: CursorTestResult): string[] => {
  if (!result.success) {
    const lines: string[] = [];
    if (result.message?.trim()) {
      lines.push(result.message);
    }
    if (result.keyTier === 'personal') {
      lines.push('Create an Admin API key under Dashboard → API Keys (admin:* scope).');
    }
    return lines;
  }

  const lines: string[] = [];
  const perms = result.permissions;

  if (result.keyTier === 'enterprise') {
    lines.push('Members, spend, usage events, and daily usage will be collected.');
    if (perms?.analytics) {
      lines.push('Enterprise analytics endpoints are accessible.');
    }
    if (perms?.aiCodeTracking) {
      lines.push('AI code tracking (commits and changes) will be collected.');
    }
    if (perms?.bugbotReviews) {
      lines.push('BugBot review analytics will be collected.');
    }
    return lines;
  }

  lines.push('Members, spend, usage events, and daily usage will be collected.');
  lines.push('AI code tracking requires an Enterprise Admin key.');
  if (perms?.bugbotReviews) {
    lines.push('BugBot review analytics will be collected.');
  }
  return lines;
};

const failureTitle = (result: CursorTestResult): string => {
  if (result.keyTier === 'personal') {
    return 'Wrong key type: Personal (User API)';
  }
  if (result.keyTier === 'team') {
    return 'Team Admin key is missing required permissions';
  }
  return 'Connection test failed';
};

const successTitle = (result: CursorTestResult): string => {
  const tier = result.keyTier ? tierLabel[result.keyTier] ?? result.keyTier : null;
  return tier ? `Detected key: ${tier}` : 'Connection test successful';
};

export const formatCursorTestMessage = (result: CursorTestResult): string => {
  if (result.message?.trim()) {
    return result.message;
  }
  return result.success ? 'Test Connection Successfully.' : 'Test Connection Failed.';
};

export const renderCursorTestSummary = (result: CursorTestResult) => {
  const lines = capabilityLines(result);

  if (!result.success) {
    if (lines.length === 0) {
      return null;
    }

    return (
      <Alert
        type="error"
        showIcon
        message={failureTitle(result)}
        description={
          <ul style={{ margin: 0, paddingLeft: 20 }}>
            {lines.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        }
      />
    );
  }

  if (lines.length === 0) {
    return null;
  }

  return (
    <Alert
      type="success"
      showIcon
      message={successTitle(result)}
      description={
        <ul style={{ margin: 0, paddingLeft: 20 }}>
          {lines.map((line) => (
            <li key={line}>{line}</li>
          ))}
        </ul>
      }
    />
  );
};

export const onCursorTestSuccess = (result: CursorTestResult, ctx: { setValues: (patch: Record<string, any>) => void }) => {
  const patch: Record<string, any> = {};
  if (result.keyTier) {
    patch.keyTier = result.keyTier;
  }
  if (result.permissions?.bugbotReviews != null) {
    patch.hasBugbotReviews = result.permissions.bugbotReviews;
  }
  if (Object.keys(patch).length > 0) {
    ctx.setValues(patch);
  }
};
