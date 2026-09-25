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
import { uniqBy } from 'lodash';
import { Alert } from 'antd';

import API from '@/api';
import type { YoutrackProjectCustomField } from '@/api/plugin/youtrack';
import { PageLoading } from '@/components';
import { useProxyPrefix, useRefreshData } from '@/hooks';

import { FieldNames, type FieldDef } from './transformation-fields/field-names';
import { MappingTable, type UnionRow } from './transformation-fields/mapping-table';

const DEFAULT_TYPE_FIELD = 'Type';
const DEFAULT_STATE_FIELD = 'State';

const TYPE_OPTIONS = ['BUG', 'REQUIREMENT', 'INCIDENT', 'TASK', 'SUBTASK'].map((value) => ({ label: value, value }));
const STATUS_OPTIONS = ['TODO', 'IN_PROGRESS', 'DONE', 'OTHER'].map((value) => ({ label: value, value }));

type LoadedProject = {
  id: string;
  label: string;
  // null marks a failed proxy call; an empty array is the []-not-403
  // permissions trap — both render a banner, never a silent empty table
  fields: YoutrackProjectCustomField[] | null;
  error?: string;
};

// unionBundle unions the values of the bundle behind the custom field named
// `fieldName` (of category `valueType`) across all attached projects, by
// value name, recording per-project presence. Ordered by first-seen bundle
// ordinal so the table reads in workflow order.
const unionBundle = (
  projects: LoadedProject[],
  valueType: string,
  fieldName: string,
): Array<UnionRow & { isResolved: boolean }> => {
  const rows = new Map<string, UnionRow & { isResolved: boolean; ordinal: number }>();
  projects.forEach((project) => {
    (project.fields ?? [])
      .filter(
        (cf) =>
          cf.field?.name === fieldName &&
          cf.field?.fieldType?.valueType === valueType &&
          !cf.field?.fieldType?.isMultiValue &&
          cf.bundle,
      )
      .forEach((cf) => {
        (cf.bundle?.values ?? []).forEach((value) => {
          const existing = rows.get(value.name);
          if (existing) {
            if (!existing.presentIn.includes(project.label)) {
              existing.presentIn.push(project.label);
            }
            existing.isResolved = existing.isResolved || !!value.isResolved;
            existing.ordinal = Math.min(existing.ordinal, value.ordinal ?? Number.MAX_SAFE_INTEGER);
          } else {
            rows.set(value.name, {
              name: value.name,
              presentIn: [project.label],
              isResolved: !!value.isResolved,
              ordinal: value.ordinal ?? Number.MAX_SAFE_INTEGER,
            });
          }
        });
      });
  });
  return [...rows.values()].sort((a, b) => a.ordinal - b.ordinal || a.name.localeCompare(b.name));
};

interface Props {
  entities: string[];
  connectionId: ID;
  scopeConfigId?: ID;
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
}

export const YoutrackTransformation = ({
  entities,
  connectionId,
  scopeConfigId,
  transformation,
  setTransformation,
}: Props) => {
  const [autoSeeds, setAutoSeeds] = useState<Record<string, string>>({});

  const prefix = useProxyPrefix({ plugin: 'youtrack', connectionId });

  // one proxy call per attached project feeds both the field-name slot
  // selects and the mapping tables; unattached configs go to manual-entry
  const { ready, data } = useRefreshData<{
    manual: boolean;
    projects: LoadedProject[];
  }>(async () => {
    if (!scopeConfigId) {
      return { manual: true, projects: [] };
    }
    const res = await API.plugin.youtrack.scopeConfigProjects(scopeConfigId);
    const attached = uniqBy(
      (res.projects ?? []).flatMap((project) => project.scopes ?? []),
      'scopeId',
    );
    if (!attached.length) {
      return { manual: true, projects: [] };
    }
    const projects = await Promise.all(
      attached.map(async (scope): Promise<LoadedProject> => {
        try {
          const fields = await API.plugin.youtrack.customFields(prefix, scope.scopeId);
          const label =
            fields.find((cf) => cf.project?.shortName)?.project?.shortName || scope.scopeName || scope.scopeId;
          return { id: scope.scopeId, label, fields };
        } catch (err: any) {
          return {
            id: scope.scopeId,
            label: scope.scopeName || scope.scopeId,
            fields: null,
            error: err?.response?.data?.message ?? err?.message ?? 'request failed',
          };
        }
      }),
    );
    return { manual: false, projects };
  }, [prefix, scopeConfigId]);

  const manual = data?.manual ?? true;
  const projects = useMemo(() => data?.projects ?? [], [data]);
  const projectLabels = useMemo(() => projects.map((project) => project.label), [projects]);

  const fieldDefs = useMemo<FieldDef[]>(
    () =>
      projects.flatMap((project) =>
        (project.fields ?? [])
          .filter((cf) => cf.field?.name)
          .map((cf) => ({
            name: cf.field!.name,
            valueType: cf.field?.fieldType?.valueType ?? '',
            isMultiValue: !!cf.field?.fieldType?.isMultiValue,
          })),
      ),
    [projects],
  );

  // the mapping tables follow the effective slot names — e.g. with the Type
  // slot set to «Client type», that field's bundle feeds the type table
  const effectiveTypeField = transformation.typeField || DEFAULT_TYPE_FIELD;
  const effectiveStateField = transformation.stateField || DEFAULT_STATE_FIELD;

  const typeRows = useMemo(
    () => (manual ? [] : unionBundle(projects, 'enum', effectiveTypeField)),
    [manual, projects, effectiveTypeField],
  );
  const stateRows = useMemo(
    () => (manual ? [] : unionBundle(projects, 'state', effectiveStateField)),
    [manual, projects, effectiveStateField],
  );

  const typeMappings: Record<string, string> = transformation.typeMappings ?? {};
  const statusMappings: Record<string, string> = transformation.statusMappings ?? {};

  // UI-side auto-seed: every unioned state not already mapped gets
  // isResolved → DONE else TODO with an `auto` badge. The seed lands in the
  // form state only — it is persisted when (and only when) the user saves.
  // Types are deliberately NOT seeded: name heuristics fail on
  // mixed-language instances.
  useEffect(() => {
    if (!stateRows.length) {
      return;
    }
    const seeds: Record<string, string> = {};
    stateRows.forEach((row) => {
      if (!(row.name in statusMappings)) {
        seeds[row.name] = row.isResolved ? 'DONE' : 'TODO';
      }
    });
    if (Object.keys(seeds).length) {
      setAutoSeeds((prev) => ({ ...prev, ...seeds }));
      setTransformation((prev: any) => ({
        ...prev,
        statusMappings: { ...(prev.statusMappings ?? {}), ...seeds },
      }));
    }
  }, [stateRows]);

  const handleChangeMapping = (key: 'typeMappings' | 'statusMappings') => (name: string, value?: string) => {
    const mappings = { ...(transformation[key] ?? {}) };
    if (value) {
      mappings[name] = value;
    } else {
      delete mappings[name];
    }
    setTransformation({ ...transformation, [key]: mappings });
  };

  if (!ready || !data) {
    return <PageLoading />;
  }

  // mapped values gone from every attached bundle are stale — kept, badge'd
  const staleTypeNames = manual
    ? []
    : Object.keys(typeMappings)
        .filter((name) => !typeRows.some((row) => row.name === name))
        .sort();
  const staleStatusNames = manual
    ? []
    : Object.keys(statusMappings)
        .filter((name) => !stateRows.some((row) => row.name === name))
        .sort();

  // manual-entry mode: the rows are the saved mappings themselves
  const manualTypeRows: UnionRow[] = manual ? Object.keys(typeMappings).map((name) => ({ name, presentIn: [] })) : [];
  const manualStatusRows: UnionRow[] = manual
    ? Object.keys(statusMappings).map((name) => ({ name, presentIn: [] }))
    : [];

  return (
    <div>
      {manual && (
        <Alert
          style={{ marginBottom: 16 }}
          type="info"
          showIcon
          message="No projects attached to this scope config yet — manual entry mode"
          description="Type field names and mapped values by hand below. Once this scope config is attached to YouTrack projects, their real field names and bundle values load here automatically through the connection."
        />
      )}
      {projects
        .filter((project) => project.fields === null)
        .map((project) => (
          <Alert
            key={project.id}
            style={{ marginBottom: 16 }}
            type="error"
            showIcon
            message={`Could not load custom fields for project ${project.label}`}
            description={project.error}
          />
        ))}
      {projects
        .filter((project) => project.fields !== null && project.fields.length === 0)
        .map((project) => (
          <Alert
            key={project.id}
            style={{ marginBottom: 16 }}
            type="warning"
            showIcon
            message={`Could not load custom fields for project ${project.label}`}
            description={
              <>
                YouTrack returned an empty list, which usually means the connection token lacks permission to read
                project custom fields — not that the project has no fields. Grant the token «Read Project» on{' '}
                {project.label}, then reload.
              </>
            }
          />
        ))}
      <p style={{ marginBottom: 0 }}>
        Which custom fields hold Type / State / Priority / Assignee / Story Points / Due Date on this instance. The
        dropdowns list type-eligible, single-value fields loaded live from the attached projects; you can also type a
        name by hand.
      </p>
      <FieldNames fieldDefs={fieldDefs} transformation={transformation} setTransformation={setTransformation} />
      {entities.includes('TICKET') && (
        <>
          <MappingTable
            title="Issue type mapping"
            description={
              manual
                ? 'Unmapped types are stored as UPPERCASE(original).'
                : `Union of the «${effectiveTypeField}» bundle values from all attached projects (by name). Unmapped types are stored as UPPERCASE(original).`
            }
            projectLabels={projectLabels}
            rows={manual ? manualTypeRows : typeRows}
            staleNames={staleTypeNames}
            mappings={typeMappings}
            options={TYPE_OPTIONS}
            unmappedOptionLabel={(name) => `(unmapped → ${name.toUpperCase()})`}
            manualMode={manual}
            emptyHint={
              manual
                ? 'No values mapped yet — add one below.'
                : `No enum field named «${effectiveTypeField}» found in the attached projects — check the Type field slot above.`
            }
            onChange={handleChangeMapping('typeMappings')}
          />
          <MappingTable
            title="Status mapping"
            description={
              manual
                ? 'Statuses map to TODO / IN_PROGRESS / DONE / OTHER.'
                : `Union of the «${effectiveStateField}» bundle values from all attached projects (by name). New values are pre-filled from the bundle's isResolved flag — review before saving.`
            }
            projectLabels={projectLabels}
            rows={manual ? manualStatusRows : stateRows}
            staleNames={staleStatusNames}
            mappings={statusMappings}
            options={STATUS_OPTIONS}
            autoSeeds={autoSeeds}
            manualMode={manual}
            emptyHint={
              manual
                ? 'No values mapped yet — add one below.'
                : `No state field named «${effectiveStateField}» found in the attached projects — check the State field slot above.`
            }
            onChange={handleChangeMapping('statusMappings')}
          />
        </>
      )}
    </div>
  );
};
