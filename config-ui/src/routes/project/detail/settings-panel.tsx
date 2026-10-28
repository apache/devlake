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

import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { CloseOutlined, PlusOutlined } from '@ant-design/icons';
import { Flex, Space, Card, Modal, Input, Checkbox, Button } from 'antd';

import API from '@/api';
import { Block, HelpTooltip, Message } from '@/components';
import { PATHS } from '@/config';
import { IProject } from '@/types';
import { operator } from '@/utils';

import { AiReviewScopeConfigModal } from './aireview-scope-config-modal';
import * as S from './styled';

const RegexPrIssueDefaultValue = '(?mi)(Closes)[\\s]*.*(((and )?#\\d+[ ]*)+)';

interface ISubProject {
  name: string;
  // Comma-separated in the UI; split into an array on save.
  prLabels: string;
  deployJobPattern: string;
}

const emptySubProject: ISubProject = { name: '', prLabels: '', deployJobPattern: '' };

// Mirrors the backend's reserved names (backend/plugins/monorepo/tasks/task_data.go):
// 'unattributed' is the sentinel written for unmatched PRs/deployments, and 'All' is the
// label dashboards show for rows with no sub_project at all. Configuring a sub-project
// with either name would make it indistinguishable from that sentinel in the UI.
const RESERVED_SUB_PROJECT_NAMES = ['unattributed', 'All'];

interface Props {
  project: IProject;
  onRefresh: () => void;
}

export const SettingsPanel = ({ project, onRefresh }: Props) => {
  const [name, setName] = useState('');
  const [dora, setDora] = useState({
    enable: false,
    prHeadRefPattern: '',
  });
  const [linker, setLinker] = useState({
    enable: false,
    prToIssueRegexp: '',
  });
  const [issueTrace, setIssueTrace] = useState({
    enable: false,
  });
  const [monorepo, setMonorepo] = useState<{ enable: boolean; subProjects: ISubProject[] }>({
    enable: false,
    subProjects: [emptySubProject],
  });
  const [aiReview, setAiReview] = useState({
    enable: false,
    scopeConfigId: undefined as number | undefined,
  });
  const [aiReviewModalOpen, setAiReviewModalOpen] = useState(false);
  const [operating, setOperating] = useState(false);
  const [open, setOpen] = useState(false);

  const navigate = useNavigate();

  useEffect(() => {
    const dora = project.metrics.find((ms) => ms.pluginName === 'dora');
    const linker = project.metrics.find((ms) => ms.pluginName === 'linker');
    const issueTrace = project.metrics.find((ms) => ms.pluginName === 'issue_trace');
    const monorepo = project.metrics.find((ms) => ms.pluginName === 'monorepo');
    const aireview = project.metrics.find((ms) => ms.pluginName === 'aireview');

    setName(project.name);
    setDora({
      enable: dora?.enable ?? false,
      prHeadRefPattern: dora?.pluginOption?.prHeadRefPattern ?? '',
    });
    setLinker({
      enable: linker?.enable ?? false,
      prToIssueRegexp: linker?.pluginOption?.prToIssueRegexp ?? RegexPrIssueDefaultValue,
    });
    setIssueTrace({
      enable: issueTrace?.enable ?? false,
    });
    const subProjects = monorepo?.pluginOption?.subProjects;
    setMonorepo({
      enable: monorepo?.enable ?? false,
      subProjects:
        Array.isArray(subProjects) && subProjects.length
          ? subProjects.map((sp: any) => ({
              name: sp.name ?? '',
              prLabels: Array.isArray(sp.prLabels) ? sp.prLabels.join(',') : '',
              deployJobPattern: sp.deployJobPattern ?? '',
            }))
          : [emptySubProject],
    });
    setAiReview({
      enable: aireview?.enable ?? false,
      scopeConfigId: aireview?.pluginOption?.scopeConfigId,
    });
  }, [project]);

  const handleAddSubProject = () => {
    setMonorepo({ ...monorepo, subProjects: [...monorepo.subProjects, { ...emptySubProject }] });
  };

  const handleDeleteSubProject = (index: number) => {
    setMonorepo({ ...monorepo, subProjects: monorepo.subProjects.filter((_, i) => i !== index) });
  };

  const handleUpdateSubProject = (index: number, field: keyof ISubProject, value: string) => {
    setMonorepo({
      ...monorepo,
      subProjects: monorepo.subProjects.map((sp, i) => (i === index ? { ...sp, [field]: value } : sp)),
    });
  };

  // Blank rows are filtered out on save (see handleUpdate), so only named rows are checked.
  const reservedSubProjectName = monorepo.enable
    ? monorepo.subProjects.map((sp) => sp.name.trim()).find((n) => RESERVED_SUB_PROJECT_NAMES.includes(n))
    : undefined;

  const handleUpdate = async () => {
    const [success] = await operator(
      () =>
        API.project.update(project.name, {
          name,
          description: '',
          metrics: [
            {
              pluginName: 'dora',
              pluginOption: {
                prHeadRefPattern: dora.prHeadRefPattern.trim(),
              },
              enable: dora.enable,
            },
            {
              pluginName: 'linker',
              pluginOption: {
                prToIssueRegexp: linker.prToIssueRegexp,
              },
              enable: linker.enable,
            },
            {
              pluginName: 'issue_trace',
              pluginOption: {},
              enable: issueTrace.enable,
            },
            {
              pluginName: 'monorepo',
              pluginOption: {
                subProjects: monorepo.subProjects
                  .filter((sp) => sp.name.trim())
                  .map((sp) => ({
                    name: sp.name.trim(),
                    prLabels: sp.prLabels
                      .split(',')
                      .map((l) => l.trim())
                      .filter((l) => l),
                    deployJobPattern: sp.deployJobPattern.trim(),
                  })),
              },
              enable: monorepo.enable,
            },
            {
              pluginName: 'aireview',
              pluginOption: {
                scopeConfigId: aiReview.scopeConfigId,
              },
              enable: aiReview.enable,
            },
          ],
        }),
      {
        setOperating,
      },
    );

    if (success) {
      onRefresh();
      navigate(PATHS.PROJECT(name), {
        state: {
          tabId: 'settings',
        },
      });
    }
  };

  const handleShowDeleteDialog = () => {
    setOpen(true);
  };

  const handleHideDeleteDialog = () => {
    setOpen(false);
  };

  const handleDelete = async () => {
    const [success] = await operator(() => API.project.remove(project.name), {
      setOperating,
      formatMessage: () => 'Delete project successful.',
    });

    if (success) {
      navigate(PATHS.PROJECTS());
    }
  };

  return (
    <Flex vertical>
      <Space direction="vertical" size="large">
        <Card>
          <Block title="Project Name" description="Edit your project name with letters, numbers, -, _ or /" required>
            <Input style={{ width: 386 }} value={name} onChange={(e) => setName(e.target.value)} />
          </Block>
          <Block
            title={
              <Checkbox checked={dora.enable} onChange={(e) => setDora({ ...dora, enable: e.target.checked })}>
                Enable DORA Metrics
              </Checkbox>
            }
            description={
              <span>
                DORA metrics are four widely-adopted metrics for measuring software delivery performance. Optionally,
                only count pull requests whose source branch matches this regex as changes for Lead Time for Changes and
                the PR cycle time metrics. Leave empty to count all merged pull requests.
                <HelpTooltip
                  overlayInnerStyle={{ width: 500 }}
                  content={
                    <>
                      <div>Example 1 - Only feature and fix branches: ^(feat|fix)/</div>
                      <div>Example 2 - Same, case-insensitive, plus hotfix: (?i)^(feat|fix|hotfix)/</div>
                      <div>Go RE2 syntax; lookahead such as (?!...) is not supported.</div>
                      <div>The refs/heads/ prefix (Azure DevOps) is ignored when matching.</div>
                    </>
                  }
                />
              </span>
            }
          >
            {dora.enable && (
              <Input
                style={{ width: 600 }}
                placeholder="^(feat|fix)/"
                value={dora.prHeadRefPattern}
                onChange={(e) => setDora({ ...dora, prHeadRefPattern: e.target.value })}
              />
            )}
          </Block>
          <Block
            title={
              <Checkbox checked={linker.enable} onChange={(e) => setLinker({ ...linker, enable: e.target.checked })}>
                Associate pull requests with issues
              </Checkbox>
            }
            description={
              <span>
                Parse the issue key with the regex from the title and description of the pull requests in this project.
                <HelpTooltip
                  overlayInnerStyle={{ width: 500 }}
                  content={
                    <>
                      <div>
                        Example 1 - If your PR title or description contains a Jira issue key in the format 'Closes
                        [DI-123](www.yourdomain.atlassian.net/browse/di-123)', please use the following regex template:{' '}
                        (?mi)Closes[\s]*.*(((and)?https://\S+.atlassian.net/browse/\S+[ ]*)+)
                      </div>
                      <div>
                        Example 2 - If your PR title or description contains a GitHub issue key in the format 'Resolves
                        www.github.com/namespace/repo_name/issues/123)', please use the following regex template:{' '}
                        (?mi)Resolves[\s]*.*(((and)?https://github.com/%s/issues/\d+[ ]*)+)
                      </div>
                    </>
                  }
                />
              </span>
            }
          >
            {linker.enable && (
              <Input
                style={{ width: 600 }}
                placeholder={RegexPrIssueDefaultValue}
                value={linker.prToIssueRegexp}
                onChange={(e) => setLinker({ ...linker, prToIssueRegexp: e.target.value })}
              />
            )}
          </Block>
          <Block
            title={
              <Checkbox checked={issueTrace.enable} onChange={(e) => setIssueTrace({ enable: e.target.checked })}>
                Enable issue trace
              </Checkbox>
            }
            description="Parse the issue status and assignee history from issue changelogs. Currently, only Jira issues are supported."
          />
          <Block
            title={
              <Checkbox
                checked={monorepo.enable}
                onChange={(e) => setMonorepo({ ...monorepo, enable: e.target.checked })}
              >
                Enable Monorepo Sub-Projects
              </Checkbox>
            }
            description={
              <span>
                Split a single repository into several logical sub-projects for DORA-style metrics. Deployments are
                matched by CI job name, pull requests by label. When a pull request carries more than one
                sub-project's label, the first matching sub-project in the list below wins.
                <HelpTooltip content="Only Deployment Frequency and Lead Time for Changes are computed. Change Failure Rate and Time to Restore Service require incident data, which sub-projects do not attribute." />
              </span>
            }
          >
            {monorepo.enable && (
              <S.SubProjectList>
                {monorepo.subProjects.map((sp, i) => (
                  <div className="row" key={i}>
                    <Input
                      style={{ width: 180 }}
                      placeholder="Sub-project name"
                      value={sp.name}
                      onChange={(e) => handleUpdateSubProject(i, 'name', e.target.value)}
                    />
                    <Input
                      style={{ width: 260 }}
                      placeholder="Labels, comma-separated, e.g. serviceA,svc-a"
                      value={sp.prLabels}
                      onChange={(e) => handleUpdateSubProject(i, 'prLabels', e.target.value)}
                    />
                    <Input
                      style={{ width: 220 }}
                      placeholder="Deploy job pattern, e.g. ^deploy-serviceA$"
                      value={sp.deployJobPattern}
                      onChange={(e) => handleUpdateSubProject(i, 'deployJobPattern', e.target.value)}
                    />
                    {monorepo.subProjects.length > 1 && (
                      <Button icon={<CloseOutlined />} onClick={() => handleDeleteSubProject(i)} />
                    )}
                  </div>
                ))}
                <Button icon={<PlusOutlined />} onClick={handleAddSubProject}>
                  Add Sub-Project
                </Button>
                {reservedSubProjectName && (
                  <Message
                    content={`"${reservedSubProjectName}" is a reserved name and cannot be used as a sub-project name.`}
                  />
                )}
              </S.SubProjectList>
            )}
          </Block>
          <Block
            title={
              <Checkbox
                checked={aiReview.enable}
                onChange={(e) => setAiReview({ ...aiReview, enable: e.target.checked })}
              >
                Enable AI Review Analysis
              </Checkbox>
            }
            description="Extract AI code review data (CodeRabbit, Qodo, Gemini, etc.) from pull request comments and calculate failure prediction metrics against CI test outcomes."
          >
            {aiReview.enable && (
              <Button onClick={() => setAiReviewModalOpen(true)}>
                {aiReview.scopeConfigId ? 'Edit Configuration' : 'Configure'}
              </Button>
            )}
          </Block>
          <Block>
            <Button type="primary" loading={operating} disabled={!name || !!reservedSubProjectName} onClick={handleUpdate}>
              Save
            </Button>
          </Block>
        </Card>
        <Flex justify="center">
          <Button type="primary" danger onClick={handleShowDeleteDialog}>
            Delete Project
          </Button>
        </Flex>
      </Space>
      {aiReviewModalOpen && (
        <AiReviewScopeConfigModal
          scopeConfigId={aiReview.scopeConfigId}
          onCancel={() => setAiReviewModalOpen(false)}
          onSave={(id) => {
            setAiReview({ ...aiReview, scopeConfigId: id });
            setAiReviewModalOpen(false);
          }}
        />
      )}
      <Modal
        open={open}
        width={820}
        centered
        title="Are you sure you want to delete this Project?"
        okText="Confirm"
        okButtonProps={{
          loading: operating,
        }}
        onCancel={handleHideDeleteDialog}
        onOk={handleDelete}
      >
        <S.DialogBody>
          <Message content="This operation cannot be undone. Deleting this project will remove all associated project settings and data. This action does not delete any data connections or the data collected through them." />
        </S.DialogBody>
      </Modal>
    </Flex>
  );
};
