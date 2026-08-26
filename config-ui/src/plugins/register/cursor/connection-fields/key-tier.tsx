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

import { Tag } from 'antd';

import { Block } from '@/components';

import { getCursorTierLabel } from '../connection-meta';

interface Props {
  type: 'create' | 'update';
  initialValues: any;
  values: any;
}

export const KeyTier = ({ type, initialValues, values }: Props) => {
  const keyTier = values.keyTier ?? initialValues.keyTier;
  const label = getCursorTierLabel(keyTier);

  if (!label) {
    if (type === 'create') {
      return null;
    }

    return (
      <Block
        title="Key Type"
        description="Detected from the API key when the connection is saved or tested. Test the connection to detect the key type."
      >
        <span style={{ color: 'rgba(0, 0, 0, 0.45)' }}>Unknown — test connection to detect</span>
      </Block>
    );
  }

  return (
    <Block
      title="Key Type"
      description="Detected from the API key when the connection is saved or tested. Re-test after changing the API key."
    >
      <Tag color={keyTier === 'enterprise' ? 'purple' : keyTier === 'personal' ? 'orange' : 'blue'}>{label}</Tag>
    </Block>
  );
};
