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

import { useEffect, useMemo, useState } from 'react';
import { Checkbox, Form, Select } from 'antd';

import { Block } from '@/components';

enum StandardType {
  Requirement = 'REQUIREMENT',
  Bug = 'BUG',
  Incident = 'INCIDENT',
}

enum StandardStatus {
  Todo = 'TODO',
  InProgress = 'IN_PROGRESS',
  Done = 'DONE',
}

const DEFAULT_TRACKER_NAMES = ['Bug', 'Feature', 'Support', 'Task'];
const DEFAULT_STATUS_NAMES = ['New', 'In Progress', 'Feedback', 'Resolved', 'Closed', 'Rejected'];

const OPTIONAL_COLLECT_FIELDS = [
  {
    key: 'collectJournals',
    label: 'Journals',
    description: 'Issue notes as comments and field changes as issue changelogs',
    defaultEnabled: false,
  },
  {
    key: 'collectTimeEntries',
    label: 'Time entries',
    description: 'Logged hours as issue worklogs. Requires View time entries permission in Redmine',
    defaultEnabled: false,
  },
] as const;

const isFieldEnabled = (transformation: any, key: string, defaultEnabled: boolean) =>
  transformation[key] ?? defaultEnabled;

interface Props {
  entities: string[];
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
}

const namesForStandardType = (typeMappings: Record<string, any> | undefined, standardType: StandardType) =>
  Object.entries(typeMappings ?? {})
    .filter(([, value]) => (value?.standardType ?? value?.standard_type) === standardType)
    .map(([name]) => name);

const namesForStandardStatus = (statusMappings: Record<string, any> | undefined, standardStatus: StandardStatus) =>
  Object.entries(statusMappings ?? {})
    .filter(([, value]) => (value?.standardStatus ?? value?.standard_status) === standardStatus)
    .map(([name]) => name);

const buildTypeMappings = (requirements: string[], bugs: string[], incidents: string[]) => ({
  ...Object.fromEntries(requirements.map((name) => [name, { standardType: StandardType.Requirement }])),
  ...Object.fromEntries(bugs.map((name) => [name, { standardType: StandardType.Bug }])),
  ...Object.fromEntries(incidents.map((name) => [name, { standardType: StandardType.Incident }])),
});

const buildStatusMappings = (todo: string[], inProgress: string[], done: string[]) => ({
  ...Object.fromEntries(todo.map((name) => [name, { standardStatus: StandardStatus.Todo }])),
  ...Object.fromEntries(inProgress.map((name) => [name, { standardStatus: StandardStatus.InProgress }])),
  ...Object.fromEntries(done.map((name) => [name, { standardStatus: StandardStatus.Done }])),
});

const selectOptions = (defaults: string[], selected: string[]) =>
  [...new Set([...defaults, ...selected])].sort().map((name) => ({ label: name, value: name }));

export const RedmineTransformation = ({ entities, transformation, setTransformation }: Props) => {
  const [requirements, setRequirements] = useState<string[]>([]);
  const [bugs, setBugs] = useState<string[]>([]);
  const [incidents, setIncidents] = useState<string[]>([]);
  const [todoStatuses, setTodoStatuses] = useState<string[]>([]);
  const [inProgressStatuses, setInProgressStatuses] = useState<string[]>([]);
  const [doneStatuses, setDoneStatuses] = useState<string[]>([]);

  useEffect(() => {
    setRequirements(namesForStandardType(transformation.typeMappings, StandardType.Requirement));
    setBugs(namesForStandardType(transformation.typeMappings, StandardType.Bug));
    setIncidents(namesForStandardType(transformation.typeMappings, StandardType.Incident));
    setTodoStatuses(namesForStandardStatus(transformation.statusMappings, StandardStatus.Todo));
    setInProgressStatuses(namesForStandardStatus(transformation.statusMappings, StandardStatus.InProgress));
    setDoneStatuses(namesForStandardStatus(transformation.statusMappings, StandardStatus.Done));
  }, [transformation]);

  const trackerOptions = useMemo(
    () => selectOptions(DEFAULT_TRACKER_NAMES, [...requirements, ...bugs, ...incidents]),
    [requirements, bugs, incidents],
  );

  const statusOptions = useMemo(
    () => selectOptions(DEFAULT_STATUS_NAMES, [...todoStatuses, ...inProgressStatuses, ...doneStatuses]),
    [todoStatuses, inProgressStatuses, doneStatuses],
  );

  const updateTypeMappings = (nextRequirements: string[], nextBugs: string[], nextIncidents: string[]) => {
    setTransformation({
      ...transformation,
      typeMappings: buildTypeMappings(nextRequirements, nextBugs, nextIncidents),
    });
  };

  const updateStatusMappings = (nextTodo: string[], nextInProgress: string[], nextDone: string[]) => {
    setTransformation({
      ...transformation,
      statusMappings: buildStatusMappings(nextTodo, nextInProgress, nextDone),
    });
  };

  if (!entities.includes('TICKET')) {
    return null;
  }

  return (
    <>
      <Block
        title="Issue type mapping"
        description="Map Redmine trackers used for requirements, bugs, and incidents. Any other tracker uses plugin defaults, then TASK automatically."
      >
        <Form labelCol={{ span: 5 }}>
          <Form.Item label="Requirement">
            <Select
              mode="tags"
              options={trackerOptions}
              value={requirements}
              onChange={(value) => updateTypeMappings(value, bugs, incidents)}
            />
          </Form.Item>
          <Form.Item label="Bug">
            <Select
              mode="tags"
              options={trackerOptions}
              value={bugs}
              onChange={(value) => updateTypeMappings(requirements, value, incidents)}
            />
          </Form.Item>
          <Form.Item label="Incident">
            <Select
              mode="tags"
              options={trackerOptions}
              value={incidents}
              onChange={(value) => updateTypeMappings(requirements, bugs, value)}
            />
          </Form.Item>
        </Form>
      </Block>

      <Block
        title="Issue status mapping"
        description="Map Redmine status names to DevLake standard statuses. Any status not mapped below uses plugin defaults, then OTHER automatically."
      >
        <Form labelCol={{ span: 5 }}>
          <Form.Item label="To do">
            <Select
              mode="tags"
              options={statusOptions}
              value={todoStatuses}
              onChange={(value) => updateStatusMappings(value, inProgressStatuses, doneStatuses)}
            />
          </Form.Item>
          <Form.Item label="In progress">
            <Select
              mode="tags"
              options={statusOptions}
              value={inProgressStatuses}
              onChange={(value) => updateStatusMappings(todoStatuses, value, doneStatuses)}
            />
          </Form.Item>
          <Form.Item label="Done">
            <Select
              mode="tags"
              options={statusOptions}
              value={doneStatuses}
              onChange={(value) => updateStatusMappings(todoStatuses, inProgressStatuses, value)}
            />
          </Form.Item>
        </Form>
      </Block>

      <Block
        title="Optional issue data"
        description="Core issue fields are always collected. Enable additional data only when needed."
      >
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
    </>
  );
};
