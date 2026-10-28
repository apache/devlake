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

import { useState, useEffect } from 'react';
import { uniqWith } from 'lodash';
import { CaretRightOutlined } from '@ant-design/icons';
import { theme, Collapse, Tag, Form, Select, Input } from 'antd';

import API from '@/api';
import { PageLoading, HelpTooltip, ExternalLink } from '@/components';
import { useProxyPrefix, useRefreshData } from '@/hooks';
import { DOC_URL } from '@/release';

import { CrossDomain } from './transformation-fields';

enum StandardType {
  Requirement = 'REQUIREMENT',
  Bug = 'BUG',
  Incident = 'INCIDENT',
}

interface Props {
  entities: string[];
  connectionId: ID;
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
}

export const JiraTransformation = ({ entities, connectionId, transformation, setTransformation }: Props) => {
  const [requirements, setRequirements] = useState<string[]>([]);
  const [bugs, setBugs] = useState<string[]>([]);
  const [incidents, setIncidents] = useState<string[]>([]);

  const prefix = useProxyPrefix({ plugin: 'jira', connectionId });

  const { ready, data } = useRefreshData<{
    issueTypes: Array<{
      id: string;
      name: string;
      iconUrl: string;
    }>;
    fields: Array<{
      id: string;
      name: string;
    }>;
  }>(async () => {
    if (!prefix) {
      return {
        issueTypes: [],
        fields: [],
      };
    }

    const [issueTypes, fields] = await Promise.all([API.plugin.jira.issueType(prefix), API.plugin.jira.field(prefix)]);
    return {
      issueTypes: uniqWith(issueTypes, (it, oit) => it.name === oit.name),
      fields,
    };
  }, [prefix]);

  useEffect(() => {
    const types = Object.entries(transformation.typeMappings ?? {}).map(([key, value]: any) => ({
      name: key,
      ...value,
    }));

    setRequirements(types.filter((it) => it.standardType === StandardType.Requirement).map((it) => it.name));
    setBugs(types.filter((it) => it.standardType === StandardType.Bug).map((it) => it.name));
    setIncidents(types.filter((it) => it.standardType === StandardType.Incident).map((it) => it.name));
  }, [transformation]);

  const { token } = theme.useToken();

  if (!ready || !data) {
    return <PageLoading />;
  }

  const { issueTypes, fields } = data;

  const transformaType = (its: string[], standardType: StandardType) => {
    return its.reduce((acc, cur) => {
      acc[cur] = {
        standardType,
      };
      return acc;
    }, {} as any);
  };

  const panelStyle: React.CSSProperties = {
    marginBottom: 24,
    background: token.colorFillAlter,
    borderRadius: token.borderRadiusLG,
    border: 'none',
  };

  return (
    <Collapse
      bordered={false}
      defaultActiveKey={['TICKET', 'CROSS']}
      expandIcon={({ isActive }) => <CaretRightOutlined rotate={isActive ? 90 : 0} />}
      style={{ background: token.colorBgContainer }}
      size="large"
      items={renderCollapseItems({
        entities,
        panelStyle,
        transformation,
        onChangeTransformation: setTransformation,
        connectionId,
        issueTypes,
        fields,
        requirements,
        bugs,
        incidents,
        transformaType,
      })}
    />
  );
};

const renderCollapseItems = ({
  entities,
  panelStyle,
  transformation,
  onChangeTransformation,
  connectionId,
  issueTypes,
  fields,
  requirements,
  bugs,
  incidents,
  transformaType,
}: {
  entities: string[];
  panelStyle: React.CSSProperties;
  transformation: any;
  onChangeTransformation: any;
  connectionId: ID;
  issueTypes: Array<{
    id: string;
    name: string;
  }>;
  fields: Array<{
    id: string;
    name: string;
  }>;
  requirements: string[];
  bugs: string[];
  incidents: string[];
  transformaType: any;
}) =>
  [
    {
      key: 'TICKET',
      label: 'Issue Tracking',
      style: panelStyle,
      children: (
        <Form labelCol={{ span: 5 }}>
          <p>
            Tell DevLake what types of Jira issues you are using as features, bugs and incidents, and what fields to use
            for `Story Points` and `Epic Link` (parent link).
          </p>
          <p>
            DevLake defines three standard types of issues: FEATURE, BUG and INCIDENT. Standardize your Jira issue types
            to these three types so that DevLake can calculate metrics such as{' '}
            <ExternalLink link={DOC_URL.METRICS.REQUIREMENT_LEAD_TIME}>Requirement Lead Time</ExternalLink>,{' '}
            <ExternalLink link={DOC_URL.METRICS.BUG_AGE}>Bug Age</ExternalLink>,
            <ExternalLink link={DOC_URL.METRICS.MTTR}>DORA - Median Time to Restore Service</ExternalLink>, etc.
          </p>
          <Form.Item label="Requirement">
            <Select
              mode="multiple"
              options={issueTypes.map((it) => ({ label: it.name, value: it.name }))}
              value={requirements}
              onChange={(value) =>
                onChangeTransformation({
                  ...transformation,
                  typeMappings: {
                    ...transformaType(value, StandardType.Requirement),
                    ...transformaType(bugs, StandardType.Bug),
                    ...transformaType(incidents, StandardType.Incident),
                  },
                })
              }
            />
          </Form.Item>
          <Form.Item label="Bug">
            <Select
              mode="multiple"
              options={issueTypes.map((it) => ({ label: it.name, value: it.name }))}
              value={bugs}
              onChange={(value) =>
                onChangeTransformation({
                  ...transformation,
                  typeMappings: {
                    ...transformaType(requirements, StandardType.Requirement),
                    ...transformaType(value, StandardType.Bug),
                    ...transformaType(incidents, StandardType.Incident),
                  },
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
            <Select
              mode="multiple"
              options={issueTypes.map((it) => ({ label: it.name, value: it.name }))}
              value={incidents}
              onChange={(value) =>
                onChangeTransformation({
                  ...transformation,
                  typeMappings: {
                    ...transformaType(requirements, StandardType.Requirement),
                    ...transformaType(bugs, StandardType.Bug),
                    ...transformaType(value, StandardType.Incident),
                  },
                })
              }
            />
          </Form.Item>
          <Form.Item
            label={
              <>
                <span>Story Points</span>
                <HelpTooltip content="Choose the issue field you are using as `Story Points`." />
              </>
            }
          >
            <Select
              showSearch
              options={fields.map((it) => ({ label: it.name, value: it.id }))}
              optionFilterProp="children"
              filterOption={(input: string, option?: { label: string; value: string }) =>
                (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
              value={transformation.storyPointField}
              onChange={(value) =>
                onChangeTransformation({
                  ...transformation,
                  storyPointField: value,
                })
              }
            />
          </Form.Item>
          <Form.Item
            label={
              <>
                <span>Extra JQL</span>
                <HelpTooltip content="Additional JQL clause ANDed into every issue query. Supports Go text/template syntax. Available variables: {{.BoardName}} (Jira board display name), {{.BoardId}} (numeric board ID), {{.ProjectName}} (DevLake project name)." />
              </>
            }
            extra='Tip: use Go template variables to make this dynamic, e.g. owner = "{{.BoardName}}"'
          >
            <Input
              placeholder='e.g. component = "my-project"'
              value={transformation.extraJql}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  extraJql: e.target.value,
                })
              }
            />
          </Form.Item>
          <Form.Item
            label={
              <>
                <span>Epic Link</span>
                <HelpTooltip content="Choose the issue field you are using as `Epic Link` or `Parent Link`. This is used to link issues to their parent epic/feature. For Red Hat Jira, this is typically 'Parent Link' (customfield_12313140)." />
              </>
            }
          >
            <Select
              showSearch
              allowClear
              placeholder="Select epic link field"
              options={fields.map((it) => ({ label: it.name, value: it.id }))}
              optionFilterProp="children"
              filterOption={(input: string, option?: { label: string; value: string }) =>
                (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
              value={transformation.epicKeyField}
              onChange={(value) =>
                onChangeTransformation({
                  ...transformation,
                  epicKeyField: value,
                })
              }
            />
          </Form.Item>
        </Form>
      ),
    },
    {
      key: 'CROSS',
      label: 'Cross Domain',
      style: panelStyle,
      children: (
        <CrossDomain
          connectionId={connectionId}
          transformation={transformation}
          setTransformation={onChangeTransformation}
        />
      ),
    },
  ].filter((it) => entities.includes(it.key));
