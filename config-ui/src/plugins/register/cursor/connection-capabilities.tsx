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

export type CursorCapabilityStatus = 'enabled' | 'disabled' | 'unavailable';

export interface CursorCapability {
  key: string;
  label: string;
  shortLabel: string;
  status: CursorCapabilityStatus;
  statusLabel: string;
}

export interface CursorCapabilitySource {
  keyTier?: string;
  hasBugbotReviews?: boolean;
  hasConversationInsights?: boolean;
}

const capabilityStatusLabel: Record<CursorCapabilityStatus, string> = {
  enabled: 'Will collect',
  disabled: 'Not available',
  unavailable: 'Unavailable for this key',
};

export const getCursorCapabilityStatus = (
  enabled: boolean | undefined,
  available: boolean,
): CursorCapabilityStatus => {
  if (!available) {
    return 'unavailable';
  }
  return enabled === true ? 'enabled' : 'disabled';
};

export const getCursorCapabilities = (source: CursorCapabilitySource): CursorCapability[] => {
  const keyTier = source.keyTier;
  if (!keyTier || keyTier === 'personal') {
    return [];
  }

  const enterpriseOptional = keyTier === 'enterprise';
  const bugbotStatus = getCursorCapabilityStatus(source.hasBugbotReviews, enterpriseOptional);
  const insightsStatus = getCursorCapabilityStatus(
    source.hasConversationInsights,
    enterpriseOptional,
  );

  return [
    {
      key: 'bugbotReviews',
      label: 'BugBot review analytics',
      shortLabel: 'BugBot',
      status: bugbotStatus,
      statusLabel: capabilityStatusLabel[bugbotStatus],
    },
    {
      key: 'conversationInsights',
      label: 'Conversation Insights',
      shortLabel: 'Insights',
      status: insightsStatus,
      statusLabel: capabilityStatusLabel[insightsStatus],
    },
  ];
};

const capabilityTagColor = (status: CursorCapabilityStatus): string | undefined => {
  switch (status) {
    case 'enabled':
      return 'green';
    case 'unavailable':
      return 'orange';
    default:
      return undefined;
  }
};

export const renderCursorCapabilityStatusTag = (capability: CursorCapability) => (
  <Tag color={capabilityTagColor(capability.status)} title={`${capability.label}: ${capability.statusLabel}`}>
    {capability.statusLabel}
  </Tag>
);

export const renderCursorCapabilityCompactTag = (capability: CursorCapability) => (
  <Tag
    color={capabilityTagColor(capability.status)}
    style={{ marginLeft: 4 }}
    title={`${capability.label}: ${capability.statusLabel}`}
  >
    {capability.shortLabel}
  </Tag>
);

interface CapabilityTagsProps {
  source: CursorCapabilitySource;
  variant: 'list' | 'detail';
}

export const CursorCapabilityTags = ({ source, variant }: CapabilityTagsProps) => {
  const capabilities = getCursorCapabilities(source);
  if (capabilities.length === 0) {
    return null;
  }

  if (variant === 'list') {
    return <>{capabilities.map((capability) => renderCursorCapabilityCompactTag(capability))}</>;
  }

  return (
    <div style={{ display: 'grid', gap: 6, marginTop: 8 }}>
      <Typography.Text type="secondary">Optional collection</Typography.Text>
      {capabilities.map((capability) => (
        <div key={capability.key}>
          <span style={{ marginRight: 8 }}>{capability.label}</span>
          {renderCursorCapabilityStatusTag(capability)}
          {capability.key === 'conversationInsights' &&
            source.keyTier === 'enterprise' &&
            capability.status === 'disabled' && (
              <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
                Enable in Cursor team settings, then re-run the pipeline.
              </Typography.Text>
            )}
        </div>
      ))}
    </div>
  );
};
