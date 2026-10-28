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

import { useState, useMemo } from 'react';
import { RedoOutlined } from '@ant-design/icons';
import { Button } from 'antd';

import API from '@/api';
import { TextTooltip } from '@/components';
import { getPluginConfig } from '@/plugins';
import { ITask, IPipelineStatus } from '@/types';
import { operator } from '@/utils';

import * as S from '../styled';

import { PipelineDuration } from './duration';

interface Props {
  task: ITask;
}

export const PipelineTask = ({ task }: Props) => {
  const [operating, setOperating] = useState(false);

  const { id, beganAt, finishedAt, status, message, progressDetail } = task;

  const [, name] = useMemo(() => {
    const config = getPluginConfig(task.plugin);
    const options = task.options;

    let name = config.name;

    switch (true) {
      case ['github', 'github_graphql'].includes(config.plugin):
        name = `${name}:${options.name}`;
        break;
      case ['gitextractor'].includes(config.plugin):
        name = `${name}:${options.name}`;
        break;
      case ['dora'].includes(config.plugin):
        name = `${name}:${options.projectName}`;
        break;
      case ['gitlab'].includes(config.plugin):
        name = `${name}:${options.projectId}`;
        break;
      case ['bitbucket'].includes(config.plugin):
        name = `${name}:${options.fullName}`;
        break;
      case ['tapd'].includes(config.plugin):
        name = `${name}:${options.workspaceId}`;
        break;
      case ['jira'].includes(config.plugin):
        name = `${name}:${options.boardId}`;
        break;
      case ['jenkins'].includes(config.plugin):
        name = `${name}:${options.fullName}`;
        break;
      case ['sonarqube'].includes(config.plugin):
        name = `${name}:${options.projectKey}`;
        break;
      case ['zentao'].includes(config.plugin):
        if (options.projectId) {
          name = `${name}:project/${options.projectId}`;
        } else {
          name = `${name}:product/${options.productId}`;
        }
        break;
      case ['refdiff'].includes(config.plugin):
        name = `${name}:${options.repoId ?? options.projectName}`;
        break;
      case ['bamboo'].includes(config.plugin):
        name = `${name}:${options.planKey}`;
        break;
      case ['azuredevops_go'].includes(config.plugin):
        name = `ado:${options.name}`;
        break;
      case ['argocd'].includes(config.plugin):
        name = `${name}:${options.ApplicationName}`;
        break;
      case ['codecov'].includes(config.plugin):
        name = `${name}:${options.fullName}`;
        break;
    }

    return [config.icon, name];
  }, [task]);

  const handleRerun = async () => {
    const [success] = await operator(() => API.task.rertun(id), {
      setOperating,
    });

    if (success) {
      //   setVersion((v) => v + 1);
    }
  };

  return (
    <S.Task>
      <div className="info">
        <div className="title">
          {/* <img src={icon} alt="" /> */}
          <strong>Task{id}</strong>
          <span>
            <TextTooltip content={name}>{name}</TextTooltip>
          </span>
        </div>
        {[status === IPipelineStatus.CREATED, IPipelineStatus.PENDING].includes(status) && <p>Subtasks pending</p>}

        {[IPipelineStatus.ACTIVE, IPipelineStatus.RUNNING].includes(status) && (
          <p>
            Subtasks running
            <strong style={{ marginLeft: 8 }}>
              {progressDetail?.finishedSubTasks}/{progressDetail?.totalSubTasks}
            </strong>
          </p>
        )}

        {status === IPipelineStatus.COMPLETED && <p>All Subtasks completed</p>}

        {status === IPipelineStatus.FAILED && (
          <TextTooltip content={message}>
            <p className="error">Task failed: hover to view the reason</p>
          </TextTooltip>
        )}

        {status === IPipelineStatus.CANCELLED && <p>Subtasks canceled</p>}
      </div>
      <div className="duration">
        <PipelineDuration status={status} beganAt={beganAt} finishedAt={finishedAt} />
        {[
          IPipelineStatus.COMPLETED,
          IPipelineStatus.PARTIAL,
          IPipelineStatus.FAILED,
          IPipelineStatus.CANCELLED,
        ].includes(status) && <Button loading={operating} icon={<RedoOutlined />} onClick={handleRerun} />}
      </div>
    </S.Task>
  );
};
