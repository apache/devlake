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

import { Checkbox, Tag } from 'antd';

import { Block } from '@/components';

interface Props {
  entities: string[];
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
}

const OPTIONAL_COLLECT_FIELDS = [
  {
    key: 'collectAssignees',
    label: 'Assignees',
    description: 'Assigned users and groups on each ticket',
    defaultEnabled: true,
  },
  {
    key: 'collectFollowups',
    label: 'Follow-ups',
    description: 'ITIL follow-up comments as issue comments',
    defaultEnabled: false,
  },
  {
    key: 'collectTasks',
    label: 'Tasks',
    description: 'Ticket tasks as issue worklogs',
    defaultEnabled: false,
  },
  {
    key: 'collectLogs',
    label: 'Change logs',
    description: 'Ticket change history as issue changelogs',
    defaultEnabled: false,
  },
  {
    key: 'collectSolutions',
    label: 'Solutions',
    description: 'Solution text on the issue record',
    defaultEnabled: false,
  },
  {
    key: 'collectTicketLinks',
    label: 'Ticket links',
    description: 'Parent/child and related ticket links as issue relationships',
    defaultEnabled: false,
  },
] as const;

const isFieldEnabled = (transformation: any, key: string, defaultEnabled: boolean) =>
  transformation[key] ?? defaultEnabled;

export const GlpiTransformation = ({ entities, transformation, setTransformation }: Props) => {
  const setOptionalCollect = (enabled: boolean) => {
    setTransformation({
      ...transformation,
      ...Object.fromEntries(OPTIONAL_COLLECT_FIELDS.map(({ key }) => [key, enabled])),
    });
  };

  const allOptionalEnabled = OPTIONAL_COLLECT_FIELDS.every(({ key, defaultEnabled }) =>
    isFieldEnabled(transformation, key, defaultEnabled),
  );

  const allOptionalDisabled = OPTIONAL_COLLECT_FIELDS.every(
    ({ key, defaultEnabled }) => !isFieldEnabled(transformation, key, defaultEnabled),
  );

  if (!entities.includes('TICKET')) {
    return null;
  }

  return (
    <Block
      title="Optional ticket data"
      description="Core ticket fields and requester are always collected. Enable additional data only when needed."
    >
      <div style={{ marginBottom: 12 }}>
        <Tag
          style={{ marginRight: 8, cursor: 'pointer' }}
          color={allOptionalEnabled ? 'blue' : 'default'}
          onClick={() => setOptionalCollect(true)}
        >
          Enable all
        </Tag>
        <Tag
          style={{ cursor: 'pointer' }}
          color={allOptionalDisabled ? 'blue' : 'default'}
          onClick={() => setOptionalCollect(false)}
        >
          Disable all
        </Tag>
      </div>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
          gap: '12px 24px',
        }}
      >
        {OPTIONAL_COLLECT_FIELDS.map(({ key, label, description, defaultEnabled }) => (
          <div key={key}>
            <Checkbox
              checked={isFieldEnabled(transformation, key, defaultEnabled)}
              onChange={(e) =>
                setTransformation({
                  ...transformation,
                  [key]: e.target.checked,
                })
              }
            >
              {label}
            </Checkbox>
            <div style={{ marginLeft: 24, color: 'rgba(0, 0, 0, 0.45)', fontSize: 12 }}>{description}</div>
          </div>
        ))}
      </div>
    </Block>
  );
};
