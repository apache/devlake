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

import { Typography } from 'antd';

import { Block } from '@/components';

import {
  CursorCapabilityTags,
  getCursorCapabilities,
  renderCursorCapabilityStatusTag,
} from '../connection-capabilities';

interface Props {
  initialValues: any;
  values: any;
}

export const Capabilities = ({ initialValues, values }: Props) => {
  const source = {
    keyTier: values.keyTier ?? initialValues.keyTier,
    hasBugbotReviews: values.hasBugbotReviews ?? initialValues.hasBugbotReviews,
    hasConversationInsights: values.hasConversationInsights ?? initialValues.hasConversationInsights,
  };

  const capabilities = getCursorCapabilities(source);
  if (capabilities.length === 0) {
    return null;
  }

  return (
    <Block
      title="Optional Data Collection"
      description="These endpoints are re-checked at the start of each pipeline run and after Test Connection, so changes in the Cursor dashboard are picked up automatically."
    >
      <div style={{ display: 'grid', gap: 8 }}>
        {capabilities.map((capability) => (
          <div key={capability.key}>
            <span style={{ marginRight: 8 }}>{capability.label}</span>
            {renderCursorCapabilityStatusTag(capability)}
            {capability.key === 'conversationInsights' &&
              source.keyTier === 'enterprise' &&
              capability.status === 'disabled' && (
                <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
                  Enable Conversation Insights in Cursor team settings, then re-run the pipeline.
                </Typography.Text>
              )}
          </div>
        ))}
      </div>
    </Block>
  );
};
