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

import { AutoComplete } from 'antd';

// FieldDef is the union (across attached projects) of custom-field
// definitions the slot selects filter against. Eligibility mirrors the
// extractor: the slot's valueType category and non-multi-value only.
export type FieldDef = {
  name: string;
  valueType: string;
  isMultiValue: boolean;
};

export type SlotKey =
  'typeField' | 'stateField' | 'priorityField' | 'assigneeField' | 'storyPointField' | 'dueDateField';

type Slot = {
  key: SlotKey;
  label: string;
  valueTypes: string[];
  defaultName?: string;
  optional?: boolean;
};

// the six configurable field-name slots. The core four fall back
// to their default name when empty; story points and due date are off when
// empty. valueTypes are the fieldType.valueType categories whose issue-level
// $type the extractor accepts for the slot.
const SLOTS: Slot[] = [
  { key: 'typeField', label: 'Type field', valueTypes: ['enum'], defaultName: 'Type' },
  { key: 'stateField', label: 'State field', valueTypes: ['state'], defaultName: 'State' },
  { key: 'priorityField', label: 'Priority field', valueTypes: ['enum'], defaultName: 'Priority' },
  { key: 'assigneeField', label: 'Assignee field', valueTypes: ['user'], defaultName: 'Assignee' },
  { key: 'storyPointField', label: 'Story Points field (optional)', valueTypes: ['integer', 'float'], optional: true },
  { key: 'dueDateField', label: 'Due Date field (optional)', valueTypes: ['date'], optional: true },
];

interface Props {
  fieldDefs: FieldDef[];
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
}

export const FieldNames = ({ fieldDefs, transformation, setTransformation }: Props) => {
  const optionsFor = (slot: Slot) =>
    [
      ...new Set(
        fieldDefs.filter((def) => !def.isMultiValue && slot.valueTypes.includes(def.valueType)).map((def) => def.name),
      ),
    ].map((name) => ({ label: name, value: name }));

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
        gap: 12,
        marginTop: 16,
      }}
    >
      {SLOTS.map((slot) => (
        <div key={slot.key}>
          <div style={{ fontSize: 12, color: 'rgba(0, 0, 0, 0.45)', marginBottom: 2 }}>{slot.label}</div>
          <AutoComplete
            style={{ width: '100%' }}
            allowClear
            options={optionsFor(slot)}
            placeholder={slot.optional ? '(none) — off' : `Default: ${slot.defaultName}`}
            filterOption={(input, option) => (option?.value ?? '').toLowerCase().includes(input.toLowerCase())}
            value={transformation[slot.key] || undefined}
            onChange={(value) =>
              setTransformation({
                ...transformation,
                [slot.key]: value ?? '',
              })
            }
          />
        </div>
      ))}
    </div>
  );
};
