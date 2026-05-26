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
import { theme, Collapse, Tag, Form, Input } from 'antd';

import { ExternalLink } from '@/components';
import { DOC_URL } from '@/release';

interface Props {
  entities: string[];
  connectionId: ID;
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
}

export const AsanaTransformation = ({ entities, transformation, setTransformation }: Props) => {
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
      expandIcon={({ isActive }) => <CaretRightOutlined rotate={isActive ? 90 : 0} />}
      style={{ background: token.colorBgContainer }}
      size="large"
      items={[
        {
          key: 'TICKET',
          label: 'Issue Tracking',
          style: panelStyle,
          children: (
            <>
              <p>
                Tell DevLake what your Asana tags mean to view metrics such as{' '}
                <ExternalLink link={DOC_URL.METRICS.BUG_AGE}>Bug Age</ExternalLink>,{' '}
                <ExternalLink link={DOC_URL.METRICS.MTTR}>DORA - Median Time to Restore Service</ExternalLink>, etc.
              </p>
              <p style={{ marginBottom: 16 }}>
                DevLake defines three standard types of issues: REQUIREMENT, BUG and INCIDENT. Classify your Asana tasks
                using tags that match the RegEx patterns below.
              </p>
              <Form.Item label="Requirement">
                <Input
                  placeholder="(feat|feature|story|requirement)"
                  value={transformation.issueTypeRequirement ?? ''}
                  onChange={(e) =>
                    setTransformation({
                      ...transformation,
                      issueTypeRequirement: e.target.value,
                    })
                  }
                />
              </Form.Item>
              <Form.Item label="Bug">
                <Input
                  placeholder="(bug|defect|broken)"
                  value={transformation.issueTypeBug ?? ''}
                  onChange={(e) =>
                    setTransformation({
                      ...transformation,
                      issueTypeBug: e.target.value,
                    })
                  }
                />
              </Form.Item>
              <Form.Item
                label={
                  <>
                    <span>Incident</span>
                    <Tag style={{ marginLeft: 4 }} color="blue">
                      DORA
                    </Tag>
                  </>
                }
              >
                <Input
                  placeholder="(incident|outage|failure)"
                  value={transformation.issueTypeIncident ?? ''}
                  onChange={(e) =>
                    setTransformation({
                      ...transformation,
                      issueTypeIncident: e.target.value,
                    })
                  }
                />
              </Form.Item>
            </>
          ),
        },
      ].filter((it) => entities.includes(it.key))}
    />
  );
};
