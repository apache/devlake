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

import { useState, useContext, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons';
import { theme, Progress, Space, Button } from 'antd';
import styled from 'styled-components';

import API from '@/api';
import { ExternalLink } from '@/components';
import { useAutoRefresh } from '@/hooks';
import { operator } from '@/utils';

import { Logs } from './components';
import { Context } from './context';

const Wrapper = styled.div`
  margin-top: 150px;
  padding: 24px;
  background-color: #fff;
  box-shadow: 0px 2.4px 4.8px -0.8px rgba(0, 0, 0, 0.1), 0px 1.6px 8px 0px rgba(0, 0, 0, 0.07);

  .top {
    margin-bottom: 42px;
    text-align: center;

    .info {
      margin-bottom: 34px;
      font-size: 16px;
      font-weight: 600;
    }

    .tip {
      margin-bottom: 42px;
      font-size: 12px;
      color: #818388;
    }

    .action {
      margin-top: 30px;
    }
  }

  .logs {
    .tip {
      font-size: 12px;
      font-weight: 600;
      color: #70727f;
    }

    .detail {
      display: flex;
      margin-top: 12px;

      & > div {
        flex: 1;
      }
    }
  }
`;

export const DashboardURLMap: Record<string, string> = {
  github: import.meta.env.DEVLAKE_DASHBOARD_URL_GITHUB,
  gitlab: import.meta.env.DEVLAKE_DASHBOARD_URL_GITLAB,
  bitbucket: import.meta.env.DEVLAKE_DASHBOARD_URL_BITBUCKET,
  azuredevops: import.meta.env.DEVLAKE_DASHBOARD_URL_AZUREDEVOPS,
  asana: import.meta.env.DEVLAKE_DASHBOARD_URL_ASANA,
};

const getStatus = (data: any) => {
  if (!data) {
    return 'running';
  }

  switch (data.status) {
    case 'TASK_COMPLETED':
      return 'success';
    case 'TASK_PARTIAL':
      return 'partial';
    case 'TASK_FAILED':
      return 'failed';
    case 'TASK_RUNNING':
    default:
      return 'running';
  }
};

export const Step4 = () => {
  const [operating, setOperating] = useState(false);

  const navigate = useNavigate();

  const { step, records, done, projectName, plugin, setRecords } = useContext(Context);

  const record = useMemo(() => records.find((it) => it.plugin === plugin), [plugin, records]);

  const { data } = useAutoRefresh(
    async () => {
      return await API.pipeline.subTasks(record?.pipelineId as string);
    },
    [record],
    {
      cancel: (data) => {
        return !!(data && ['TASK_COMPLETED', 'TASK_PARTIAL', 'TASK_FAILED'].includes(data.status));
      },
    },
  );

  const [status, percent, collector, extractor] = useMemo(() => {
    const status = getStatus(data);
    const percent = Math.floor((data?.completionRate ?? 0) * 100);

    const collectorTask = (data?.subtasks ?? [])[0] ?? {};
    const extractorTask = (data?.subtasks ?? [])[1] ?? {};

    const collectorSubtasks = (collectorTask.subtaskDetails ?? []).filter((it) => it.isCollector);
    const collectorSubtasksFinished = collectorSubtasks.filter((it) => it.finishedAt || it.isFailed);

    const collector = {
      plugin: collectorTask.plugin,
      name: `Collect non-Git entities in ${collectorTask.options?.fullName ?? 'Unknown'}`,
      percent: collectorSubtasks.length
        ? Math.floor((collectorSubtasksFinished.length / collectorSubtasks.length) * 100)
        : 0,
      tasks: collectorSubtasks.map((it) => ({
        step: it.sequence,
        name: it.name,
        status: it.isFailed ? 'failed' : !it.beganAt ? 'pending' : it.finishedAt ? 'success' : 'running',
        finishedRecords: it.finishedRecords,
      })),
    };

    const extractorSubtasks = (extractorTask.subtaskDetails ?? []).filter((it) => it.isCollector);
    const extractorSubtasksFinished = extractorSubtasks.filter((it) => it.finishedAt || it.isFailed);

    const extractor = {
      plugin: extractorTask.plugin,
      name: `Collect Git entities in ${extractorTask.options?.fullName ?? 'Unknown'}`,
      percent: extractorSubtasks.length
        ? Math.floor((extractorSubtasksFinished.length / extractorSubtasks.length) * 100)
        : 0,
      tasks: (extractorTask.subtaskDetails ?? [])
        .filter((it) => it.isCollector)
        .map((it) => ({
          step: it.sequence,
          name: it.name,
          status: it.isFailed ? 'failed' : !it.beganAt ? 'pending' : it.finishedAt ? 'success' : 'running',
          finishedRecords: it.finishedRecords,
        })),
    };

    return [status, percent, collector, extractor];
  }, [data]);

  const {
    token: { green5, orange5, red5 },
  } = theme.useToken();

  const handleFinish = async () => {
    const [success] = await operator(
      () =>
        API.store.set('onboard', {
          step,
          records,
          done: true,
          projectName,
          plugin,
        }),
      {
        setOperating,
      },
    );

    if (success) {
      navigate('/');
    }
  };

  const handleRecollectData = async () => {
    if (!record) {
      return null;
    }

    const [success] = await operator(
      async () => {
        // 1. re trigger this bulueprint
        await API.blueprint.trigger(record.blueprintId, { skipCollectors: false, fullSync: false });

        // 2. get current run pipeline
        const pipeline = await API.blueprint.pipelines(record.blueprintId);

        const newRecords = records.map((it) =>
          it.plugin !== plugin
            ? it
            : {
                ...it,
                pipelineId: pipeline.pipelines[0].id,
              },
        );

        setRecords(newRecords);

        // 3. update store
        await API.store.set('onboard', {
          step: 4,
          records: newRecords,
          done,
          projectName,
          plugin,
        });
      },
      {
        setOperating,
      },
    );

    if (success) {
    }
  };

  if (!plugin || !record) {
    return null;
  }

  const { scopeName } = record;

  return (
    <Wrapper>
      {status === 'running' && (
        <div className="top">
          <div className="info">Syncing up data from {scopeName}...</div>
          <div className="tip">
            This may take a few minutes to hours, depending on the volume of your data and the rate limits imposed by
            the selected tool. To speed up, only data updated from the past 14 days will be collected. However, this
            timeframe can be modified at any time via the project details page.
          </div>
          <Progress type="circle" size={120} percent={percent} />
        </div>
      )}
      {status === 'success' && (
        <div className="top">
          <div className="info">{scopeName} is successfully collected !</div>
          <CheckCircleOutlined style={{ fontSize: 120, color: green5 }} />
          <div className="action">
            <Space direction="vertical">
              <Button type="primary" onClick={() => window.open(DashboardURLMap[plugin])}>
                Check Dashboard
              </Button>
              <Button loading={operating} onClick={handleFinish}>
                Finish and Exit
              </Button>
            </Space>
          </div>
        </div>
      )}
      {status === 'partial' && (
        <div className="top">
          <div className="info">Data from {scopeName} has been partially collected!</div>
          <CheckCircleOutlined style={{ fontSize: 120, color: orange5 }} />
          <div className="action">
            <Space>
              <Button type="primary" onClick={handleRecollectData}>
                Re-collect Data
              </Button>
              <Button type="primary" onClick={() => window.open(DashboardURLMap[plugin])}>
                Check Dashboard
              </Button>
            </Space>
          </div>
        </div>
      )}
      {status === 'failed' && (
        <div className="top">
          <div className="info">Something went wrong with the collection process.</div>
          <div className="tip">
            Please verify your network connection and ensure your token's rate limits have not been exceeded, then
            attempt to collect the data again. Alternatively, you may report the issue by filing a bug on{' '}
            <ExternalLink link="https://github.com/apache/incubator-devlake/issues/new/choose">GitHub</ExternalLink>.
          </div>
          <CloseCircleOutlined style={{ fontSize: 120, color: red5 }} />
          <div className="action">
            <Space direction="vertical">
              <Button type="primary" onClick={handleRecollectData}>
                Re-collect Data
              </Button>
            </Space>
          </div>
        </div>
      )}
      <div className="logs">
        <div className="tip">Data synchronization progress:</div>
        <div className="detail">
          <Logs log={collector} />
          <Logs log={extractor} style={{ marginLeft: 16 }} />
        </div>
      </div>
    </Wrapper>
  );
};
