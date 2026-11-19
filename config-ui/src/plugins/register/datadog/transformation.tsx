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
import { theme, Collapse, Input } from 'antd';

import { HelpTooltip } from '@/components';

interface Props {
  entities: string[];
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
}

export const DatadogTransformation = ({ entities, transformation, setTransformation }: Props) => {
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
      defaultActiveKey={['TICKET']}
      expandIcon={({ isActive }) => <CaretRightOutlined rotate={isActive ? 90 : 0} rev="" />}
      style={{ background: token.colorBgContainer }}
      size="large"
      items={[
        {
          key: 'TICKET',
          label: 'Incident',
          style: panelStyle,
          children: (
            <>
              <p style={{ marginBottom: 16 }}>
                Datadog mirrors its attributes and your organization-defined custom fields into each incident&apos;s{' '}
                <code>fields</code> object, so name the field that carries each value below. Leave a field blank to skip
                it.
              </p>
              <div style={{ margin: '8px 0' }}>
                <span>Map the domain component from the incident field</span>
                <Input
                  style={{ width: 200, margin: '0 8px' }}
                  placeholder="services"
                  value={transformation.componentField ?? ''}
                  onChange={(e) =>
                    setTransformation({
                      ...transformation,
                      componentField: e.target.value,
                    })
                  }
                />
                <HelpTooltip content="The Datadog incident field whose value becomes the DevLake component, e.g. `services`. Multi-select fields use their first value." />
              </div>
              <div style={{ margin: '8px 0' }}>
                <span>Override the severity from the incident field</span>
                <Input
                  style={{ width: 200, margin: '0 8px' }}
                  placeholder="(defaults to Datadog severity)"
                  value={transformation.severityField ?? ''}
                  onChange={(e) =>
                    setTransformation({
                      ...transformation,
                      severityField: e.target.value,
                    })
                  }
                />
                <HelpTooltip content="Leave blank to use Datadog's own SEV severity. Set it to a custom field name to override that with the field's value when present." />
              </div>
            </>
          ),
        },
      ].filter((it) => entities.includes(it.key))}
    />
  );
};
