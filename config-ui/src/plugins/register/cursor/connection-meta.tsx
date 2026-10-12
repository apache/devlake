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

import { Tag, Typography } from 'antd';

import { IConnection, IConnectionMetaVariant } from '@/types';

import { CursorCapabilityTags } from './connection-capabilities';

export const CURSOR_TIER_LABELS: Record<string, string> = {
  enterprise: 'Enterprise Admin',
  team: 'Team Admin',
  personal: 'Personal (User API)',
};

const tierHint: Record<string, string> = {
  enterprise: 'Team data, analytics, and AI code tracking will be collected.',
  team: 'Team data will be collected. AI code tracking requires an Enterprise Admin key.',
  personal: 'Personal User API keys cannot access team data.',
};

export const getCursorTierLabel = (keyTier?: string): string | null => {
  if (!keyTier?.trim()) {
    return null;
  }
  return CURSOR_TIER_LABELS[keyTier] ?? keyTier;
};

const tierTagColor = (keyTier: string): string => {
  switch (keyTier) {
    case 'enterprise':
      return 'purple';
    case 'personal':
      return 'orange';
    default:
      return 'blue';
  }
};

export const renderCursorConnectionMeta = (connection: IConnection, variant: IConnectionMetaVariant) => {
  const label = getCursorTierLabel(connection.keyTier);
  if (!label) {
    return null;
  }

  const capabilitySource = {
    keyTier: connection.keyTier,
    hasBugbotReviews: connection.hasBugbotReviews,
    hasConversationInsights: connection.hasConversationInsights,
  };

  if (variant === 'list') {
    return (
      <>
        <Tag color={tierTagColor(connection.keyTier!)} style={{ marginLeft: 8 }}>
          {label}
        </Tag>
        <CursorCapabilityTags source={capabilitySource} variant="list" />
      </>
    );
  }

  const hint = tierHint[connection.keyTier!];
  return (
    <div>
      <div>
        <span style={{ marginRight: 4 }}>Key type:</span>
        <Tag color={tierTagColor(connection.keyTier!)}>{label}</Tag>
        {hint && (
          <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
            {hint}
          </Typography.Text>
        )}
      </div>
      <CursorCapabilityTags source={capabilitySource} variant="detail" />
    </div>
  );
};
