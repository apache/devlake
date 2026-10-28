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

import { useEffect } from 'react';
import { CaretRightOutlined } from '@ant-design/icons';
import { Collapse, Input, Select, theme } from 'antd';

import { Block } from '@/components';

interface Props {
  entities: string[];
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
  setHasError: React.Dispatch<React.SetStateAction<boolean>>;
}

const parsers = [
  { label: 'RFE assessment (rfe_assess)', value: 'rfe_assess' },
  { label: 'Strategy pipeline (strat_pipeline)', value: 'strat_pipeline' },
];

export const AisdlcTransformation = ({ entities, transformation, setTransformation, setHasError }: Props) => {
  const { token } = theme.useToken();

  useEffect(() => {
    const missing =
      !transformation.tenantId || !transformation.sourceId || !transformation.stationId || !transformation.parser;
    setHasError(missing);
  }, [transformation, setHasError]);

  const setField = (key: string, value: string) => {
    setTransformation({ ...transformation, [key]: value });
  };

  if (!entities.includes('CODE')) {
    return null;
  }

  return (
    <Collapse
      bordered={false}
      defaultActiveKey={['CODE']}
      expandIcon={({ isActive }) => <CaretRightOutlined rotate={isActive ? 90 : 0} rev="" />}
      style={{ background: token.colorBgContainer }}
      size="large"
      items={[
        {
          key: 'CODE',
          label: 'Station',
          style: {
            marginBottom: 24,
            background: token.colorFillAlter,
            borderRadius: token.borderRadiusLG,
            border: 'none',
          },
          children: (
            <>
              <Block title="Tenant" description="Tenant that owns these observations." required>
                <Input value={transformation.tenantId ?? ''} onChange={(e) => setField('tenantId', e.target.value)} />
              </Block>
              <Block
                title="Source"
                description="Stable source id for this repository. It is part of observation identity."
                required
              >
                <Input value={transformation.sourceId ?? ''} onChange={(e) => setField('sourceId', e.target.value)} />
              </Block>
              <Block title="Station" description="Station id, for example rfe-creator or strategy-creator." required>
                <Input value={transformation.stationId ?? ''} onChange={(e) => setField('stationId', e.target.value)} />
              </Block>
              <Block title="Display name" description="Optional label. Defaults to the station id when left empty.">
                <Input
                  value={transformation.displayName ?? ''}
                  onChange={(e) => setField('displayName', e.target.value)}
                />
              </Block>
              <Block title="Parser" description="Which station files this repository contains." required>
                <Select
                  style={{ width: '100%' }}
                  placeholder="Select a parser"
                  value={transformation.parser || undefined}
                  options={parsers}
                  onChange={(value) => setField('parser', value)}
                />
              </Block>
              <Block title="Branch" description="Optional. Empty uses the repository default branch.">
                <Input value={transformation.branch ?? ''} onChange={(e) => setField('branch', e.target.value)} />
              </Block>
              <Block
                title="Path prefix"
                description="Optional directory or file prefix. Use it to collect one batch instead of the whole tree."
              >
                <Input
                  value={transformation.pathPrefix ?? ''}
                  onChange={(e) => setField('pathPrefix', e.target.value)}
                />
              </Block>
            </>
          ),
        },
      ]}
    />
  );
};
