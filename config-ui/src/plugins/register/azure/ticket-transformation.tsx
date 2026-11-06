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

const PROCESS_TEMPLATE_OPTIONS = [
  { label: 'Auto-detect from project', value: '' },
  { label: 'Basic', value: 'Basic' },
  { label: 'Agile', value: 'Agile' },
  { label: 'Scrum', value: 'Scrum' },
  { label: 'CMMI', value: 'CMMI' },
];

const DEFAULT_WORK_ITEM_TYPES = [
  'Epic',
  'Feature',
  'User Story',
  'Product Backlog Item',
  'Bug',
  'Task',
  'Issue',
  'Requirement',
  'Change Request',
  'Risk',
];

const DEFAULT_STATUS_NAMES = [
  'New',
  'Proposed',
  'To Do',
  'Active',
  'Doing',
  'Committed',
  'In Progress',
  'Resolved',
  'Closed',
  'Done',
  'Removed',
  'Cut',
];

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

const OPTIONAL_COLLECT_FIELDS = [
  {
    key: 'collectChangelogs',
    label: 'Work item changelogs',
    description:
      'Field history (status, assignee, sprint, etc.) for time-in-status and Issue Trace metrics.',
    defaultEnabled: false,
  },
  {
    key: 'collectComments',
    label: 'Work item comments',
    description:
      'Discussion comments for response-time and collaboration metrics.',
    defaultEnabled: false,
  },
] as const;

const isFieldEnabled = (transformation: any, key: string, defaultEnabled: boolean) =>
  transformation[key] ?? defaultEnabled;

interface Props {
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
}

export const AzureTicketTransformation = ({ transformation, setTransformation }: Props) => {
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
  }, [transformation.typeMappings, transformation.statusMappings]);

  const workItemTypeOptions = useMemo(
    () => selectOptions(DEFAULT_WORK_ITEM_TYPES, [...requirements, ...bugs, ...incidents]),
    [requirements, bugs, incidents],
  );

  const statusOptions = useMemo(
    () => selectOptions(DEFAULT_STATUS_NAMES, [...todoStatuses, ...inProgressStatuses, ...doneStatuses]),
    [todoStatuses, inProgressStatuses, doneStatuses],
  );

  const updateTypeMappings = (nextRequirements: string[], nextBugs: string[], nextIncidents: string[]) => {
    setRequirements(nextRequirements);
    setBugs(nextBugs);
    setIncidents(nextIncidents);
    setTransformation({
      ...transformation,
      typeMappings: buildTypeMappings(nextRequirements, nextBugs, nextIncidents),
    });
  };

  const updateStatusMappings = (nextTodo: string[], nextInProgress: string[], nextDone: string[]) => {
    setTodoStatuses(nextTodo);
    setInProgressStatuses(nextInProgress);
    setDoneStatuses(nextDone);
    setTransformation({
      ...transformation,
      statusMappings: buildStatusMappings(nextTodo, nextInProgress, nextDone),
    });
  };

  return (
    <>
      <Block
        title="Optional data collection"
        description="Core work item fields and links are always collected. Enable these only when you need history or comment metrics."
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
                onChange={(event) =>
                  setTransformation({
                    ...transformation,
                    [key]: event.target.checked,
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

      <Block title="Process template" description="Used for default work item type mapping when auto-detect is unavailable.">
        <Form.Item label="Template">
          <Select
            options={PROCESS_TEMPLATE_OPTIONS}
            value={transformation.processTemplate ?? ''}
            onChange={(value) => setTransformation({ ...transformation, processTemplate: value || null })}
          />
        </Form.Item>
      </Block>

      <Block
        title="Work item type mapping"
        description="Map Azure work item types to DevLake standard types. Defaults are applied first; overrides here take precedence."
      >
        <Form.Item label="REQUIREMENT">
          <Select
            mode="tags"
            options={workItemTypeOptions}
            value={requirements}
            onChange={(value) => updateTypeMappings(value, bugs, incidents)}
          />
        </Form.Item>
        <Form.Item label="BUG">
          <Select
            mode="tags"
            options={workItemTypeOptions}
            value={bugs}
            onChange={(value) => updateTypeMappings(requirements, value, incidents)}
          />
        </Form.Item>
        <Form.Item label="INCIDENT">
          <Select
            mode="tags"
            options={workItemTypeOptions}
            value={incidents}
            onChange={(value) => updateTypeMappings(requirements, bugs, value)}
          />
        </Form.Item>
      </Block>

      <Block
        title="Status mapping overrides"
        description="Statuses are mapped automatically from Azure state categories. Override specific state names here when needed."
      >
        <Form.Item label="TODO">
          <Select
            mode="tags"
            options={statusOptions}
            value={todoStatuses}
            onChange={(value) => updateStatusMappings(value, inProgressStatuses, doneStatuses)}
          />
        </Form.Item>
        <Form.Item label="IN_PROGRESS">
          <Select
            mode="tags"
            options={statusOptions}
            value={inProgressStatuses}
            onChange={(value) => updateStatusMappings(todoStatuses, value, doneStatuses)}
          />
        </Form.Item>
        <Form.Item label="DONE">
          <Select
            mode="tags"
            options={statusOptions}
            value={doneStatuses}
            onChange={(value) => updateStatusMappings(todoStatuses, inProgressStatuses, value)}
          />
        </Form.Item>
      </Block>
    </>
  );
};
