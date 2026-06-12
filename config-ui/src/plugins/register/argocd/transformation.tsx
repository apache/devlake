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

import { CaretRightOutlined } from '@ant-design/icons';
import { theme, Collapse, Tag, Input } from 'antd';

import { ExternalLink, HelpTooltip } from '@/components';
import { DOC_URL } from '@/release';

interface Props {
  entities: string[];
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
}

export const ArgoCDTransformation = ({ entities, transformation, setTransformation }: Props) => {
  const { token } = theme.useToken();

  const panelStyle: React.CSSProperties = {
    marginBottom: 24,
    background: token.colorFillAlter,
    borderRadius: token.borderRadiusLG,
    border: 'none',
  };

  return (
    <Collapse
      bordered={false}
      defaultActiveKey={['CICD']}
      expandIcon={({ isActive }) => <CaretRightOutlined rotate={isActive ? 90 : 0} rev="" />}
      style={{ background: token.colorBgContainer }}
      size="large"
      items={renderCollapseItems({
        entities,
        panelStyle,
        transformation,
        onChangeTransformation: setTransformation,
      })}
    />
  );
};

const renderCollapseItems = ({
  entities,
  panelStyle,
  transformation,
  onChangeTransformation,
}: {
  entities: string[];
  panelStyle: React.CSSProperties;
  transformation: any;
  onChangeTransformation: any;
}) =>
  [
    {
      key: 'CICD',
      label: 'CI/CD',
      style: panelStyle,
      children: (
        <>
          <h3 style={{ marginBottom: 16 }}>
            <span>Deployment</span>
            <Tag style={{ marginLeft: 4 }} color="blue">
              DORA
            </Tag>
          </h3>
          <p style={{ marginBottom: 16 }}>
            Use Regular Expressions to define how DevLake identifies deployments and production environments from ArgoCD
            sync operations to measure DORA metrics.
          </p>
          <div style={{ marginTop: 16 }}>
            <strong>Environment Detection</strong>
          </div>
          <div style={{ margin: '8px 0', paddingLeft: 28 }}>
            <span>An ArgoCD sync operation is a 'Production Deployment' when the application name matches</span>
            <Input
              style={{ width: 200, margin: '0 8px' }}
              placeholder="(?i)prod(.*)"
              value={transformation.envNamePattern ?? '(?i)prod(.*)'}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  envNamePattern: e.target.value,
                })
              }
            />
            <i style={{ color: 'var(--devlake-color-error-alt)' }}>*</i>
            <HelpTooltip content="Use regex to match application names. Default pattern matches any name containing 'prod' (case-insensitive)." />
          </div>
          <div style={{ marginTop: 16, marginBottom: 8 }}>
            <strong>Optional Filters</strong>
          </div>
          <div style={{ margin: '8px 0', paddingLeft: 28 }}>
            <span>Deployment Pattern (filter sync operations by application name)</span>
            <Input
              style={{ width: 200, margin: '0 8px' }}
              placeholder=".*"
              value={transformation.deploymentPattern ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  deploymentPattern: e.target.value,
                })
              }
            />
            <HelpTooltip content="Optional: Use regex to include only specific applications. Leave empty to include all." />
          </div>
          <div style={{ margin: '8px 0', paddingLeft: 28 }}>
            <span>Production Pattern (additional pattern for production detection)</span>
            <Input
              style={{ width: 200, margin: '0 8px' }}
              placeholder=""
              value={transformation.productionPattern ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  productionPattern: e.target.value,
                })
              }
            />
            <HelpTooltip content="Optional: Additional regex pattern to identify production deployments." />
          </div>
          <div style={{ marginTop: 16, padding: '8px 12px', background: '#f0f7ff', borderRadius: '4px' }}>
            <strong>Note:</strong> ArgoCD limits deployment history to the last 10 sync operations by default
            (controlled by <code>revisionHistoryLimit</code>). Consider increasing this value in your ArgoCD application
            settings for better historical metrics.
          </div>
        </>
      ),
    },
  ].filter((it) => entities.includes(it.key));
