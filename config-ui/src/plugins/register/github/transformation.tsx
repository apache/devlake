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
import { CheckCircleOutlined, CloseCircleOutlined, CaretRightOutlined } from '@ant-design/icons';
import type { CheckboxChangeEvent } from 'antd/lib/checkbox';
import { theme, Form, Collapse, Input, Tag, Checkbox } from 'antd';

import { HelpTooltip, ExternalLink } from '@/components';
import { DOC_URL } from '@/release';

interface Props {
  entities: string[];
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
  setHasError: React.Dispatch<React.SetStateAction<boolean>>;
}

export const GitHubTransformation = ({ entities, transformation, setTransformation, setHasError }: Props) => {
  const [useCustom, setUseCustom] = useState(false);

  useEffect(() => {
    if (transformation.deploymentPattern || transformation.productionPattern) {
      setUseCustom(true);
    } else {
      setUseCustom(false);
    }
  }, [transformation]);

  useEffect(() => {
    setHasError(useCustom && !transformation.deploymentPattern);
  }, [useCustom, transformation]);

  const handleChangeUseCustom = (e: CheckboxChangeEvent) => {
    const checked = (e.target as HTMLInputElement).checked;

    if (!checked) {
      setTransformation({
        ...transformation,
        deploymentPattern: '',
        productionPattern: '',
      });
    }

    setUseCustom(checked);
  };

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
      defaultActiveKey={['TICKET', 'CICD']}
      expandIcon={({ isActive }) => <CaretRightOutlined rotate={isActive ? 90 : 0} rev="" />}
      style={{ background: token.colorBgContainer }}
      size="large"
      items={renderCollapseItems({
        entities,
        panelStyle,
        transformation,
        onChangeTransformation: setTransformation,
        useCustom,
        onChangeUseCustom: handleChangeUseCustom,
      })}
    />
  );
};

const renderCollapseItems = ({
  entities,
  panelStyle,
  transformation,
  onChangeTransformation,
  useCustom,
  onChangeUseCustom,
}: {
  entities: string[];
  panelStyle: React.CSSProperties;
  transformation: any;
  onChangeTransformation: any;
  useCustom: boolean;
  onChangeUseCustom: any;
}) =>
  [
    {
      key: 'TICKET',
      label: 'Issue Tracking',
      style: panelStyle,
      children: (
        <>
          <p>
            Tell DevLake what your issue labels mean to view metrics such as{' '}
            <ExternalLink link={DOC_URL.METRICS.BUG_AGE}>Bug Age</ExternalLink>,{' '}
            <ExternalLink link={DOC_URL.METRICS.MTTR}>DORA - Median Time to Restore Service</ExternalLink>, etc.
          </p>
          <p>
            DevLake defines three standard types of issues: FEATURE, BUG and INCIDENT. Set your issues to these three
            types with issue labels that match the RegEx.
          </p>
          <Form.Item label="Requirement">
            <Input
              placeholder="(feat|feature|proposal|requirement)"
              value={transformation.issueTypeRequirement ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  issueTypeRequirement: e.target.value,
                })
              }
            />
          </Form.Item>
          <Form.Item label="Bug">
            <Input
              placeholder="(bug|broken)"
              value={transformation.issueTypeBug ?? ''}
              onChange={(e) =>
                onChangeTransformation({
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
              placeholder="(incident|failure)"
              value={transformation.issueTypeIncident ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  issueTypeIncident: e.target.value,
                })
              }
            />
          </Form.Item>
          <Form.Item
            label={
              <>
                <span style={{ marginRight: 4 }}>Issue Priority</span>
                <HelpTooltip content="Labels that match the RegEx will be set as the priority of an issue." />
              </>
            }
          >
            <Input
              placeholder="(highest|high|medium|low|p0|p1|p2|p3)"
              value={transformation.issuePriority ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  issuePriority: e.target.value,
                })
              }
            />
          </Form.Item>
          <Form.Item
            label={
              <>
                <span style={{ marginRight: 4 }}>Issue Component</span>
                <HelpTooltip content="Labels that match the RegEx will be set as the component of an issue." />
              </>
            }
          >
            <Input
              placeholder="component(.*)"
              value={transformation.issueComponent ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  issueComponent: e.target.value,
                })
              }
            />
          </Form.Item>
          <Form.Item
            label={
              <>
                <span style={{ marginRight: 4 }}>Issue Severity</span>
                <HelpTooltip content="Labels that match the RegEx will be set as the severity of an issue." />
              </>
            }
          >
            <Input
              placeholder="severity(.*)"
              value={transformation.issueSeverity ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  issueSeverity: e.target.value,
                })
              }
            />
          </Form.Item>
        </>
      ),
    },
    {
      key: 'CICD',
      label: 'CI/CD',
      style: panelStyle,
      children: (
        <>
          <h3 style={{ marginBottom: 16 }}>
            <span>Deployment</span>
            <Tag style={{ marginLeft: 4 }} color="blue">
              DORA
            </Tag>
          </h3>
          <p style={{ marginBottom: 16 }}>
            Use Regular Expression to define Deployments in DevLake in order to measure DORA metrics.{' '}
            <ExternalLink link={DOC_URL.PLUGIN.GITHUB.TRANSFORMATION}>Learn more</ExternalLink>
          </p>
          <Checkbox disabled checked>
            Convert a GitHub Deployment to a DevLake Deployment
          </Checkbox>
          <div style={{ margin: '8px 0', paddingLeft: 28 }}>
            <span>If its environment name matches</span>
            <Input
              style={{ width: 180, margin: '0 8px' }}
              placeholder="(?i)prod(.*)"
              value={transformation.envNamePattern}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  envNamePattern: e.target.value,
                })
              }
            />
            <span>, this deployment is a ‘Production Deployment’</span>
          </div>
          <Checkbox checked={useCustom} onChange={onChangeUseCustom}>
            Convert a GitHub workflow run as a DevLake Deployment when:
          </Checkbox>
          <div style={{ margin: '8px 0', paddingLeft: 28 }}>
            <span>
              The name of the <strong>GitHub workflow run</strong> or <strong> one of its jobs</strong> matches
            </span>
            <Input
              style={{ width: 180, margin: '0 8px' }}
              placeholder="(deploy|push-image)"
              value={transformation.deploymentPattern ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  deploymentPattern: e.target.value,
                  productionPattern: !e.target.value ? '' : transformation.productionPattern,
                })
              }
            />
            <i style={{ marginRight: 4, color: 'var(--devlake-color-error-alt)' }}>*</i>
            <HelpTooltip content="GitHub Workflow Runs: https://docs.github.com/en/actions/managing-workflow-runs/manually-running-a-workflow" />
          </div>
          <div style={{ margin: '8px 0', paddingLeft: 28 }}>
            <span>If the name or its branch’s name also matches</span>
            <Input
              style={{ width: 180, margin: '0 8px' }}
              placeholder="prod(.*)"
              value={transformation.productionPattern ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  productionPattern: e.target.value,
                })
              }
            />
            <span>, this deployment is a ‘Production Deployment’</span>
            <HelpTooltip content="If you leave this field empty, all Deployments will be tagged as in the Production environment. " />
          </div>
        </>
      ),
    },
    {
      key: 'CODEREVIEW',
      label: 'Code Review',
      style: panelStyle,
      children: (
        <>
          <p>
            If you use labels to identify types and components of pull requests, use the following RegExes to extract
            them into corresponding columns.{' '}
            <ExternalLink link={DOC_URL.DATA_MODELS.DEVLAKE_DOMAIN_LAYER_SCHEMA.PULL_REQUEST}>Learn More</ExternalLink>
          </p>
          <Form.Item
            label={
              <>
                <span style={{ marginRight: 4 }}>PR Type</span>
                <HelpTooltip content="Labels that match the RegEx will be set as the type of a pull request." />
              </>
            }
          >
            <Input
              placeholder="type(.*)$"
              value={transformation.prType ?? ''}
              onChange={(e) => onChangeTransformation({ ...transformation, prType: e.target.value })}
            />
          </Form.Item>
          <Form.Item
            label={
              <>
                <span style={{ marginRight: 4 }}>PR Component</span>
                <HelpTooltip content="Labels that match the RegEx will be set as the component of a pull request." />
              </>
            }
          >
            <Input
              placeholder="component(.*)$"
              value={transformation.prComponent ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  prComponent: e.target.value,
                })
              }
            />
          </Form.Item>
        </>
      ),
    },
    {
      key: 'CROSS',
      label: 'Cross-domain',
      style: panelStyle,
      children: (
        <>
          <p>
            Connect entities across domains to measure metrics such as{' '}
            <ExternalLink link={DOC_URL.METRICS.BUG_COUNT_PER_1K_LINES_OF_CODE}>
              Bug Count per 1k Lines of Code
            </ExternalLink>
            .
          </p>
          <Form.Item
            labelCol={{ span: 6 }}
            label={
              <div className="label">
                <span style={{ marginRight: 4 }}>Connect PRs and Issues</span>
                <HelpTooltip
                  content={
                    <>
                      <div>
                        <CheckCircleOutlined rev="" style={{ marginRight: 4, color: 'var(--devlake-color-success)' }} />
                        Example 1: PR #321 body contains "<strong>Closes #1234</strong>" (PR #321 and issue #1234 will
                        be mapped by the following RegEx)
                      </div>
                      <div>
                        <CloseCircleOutlined rev="" style={{ marginRight: 4, color: 'var(--devlake-color-error-alt)' }} />
                        Example 2: PR #321 body contains "<strong>Related to #1234</strong>" (PR #321 and issue #1234
                        will NOT be mapped by the following RegEx)
                      </div>
                    </>
                  }
                />
              </div>
            }
          >
            <Input.TextArea
              value={transformation.prBodyClosePattern ?? ''}
              placeholder="(?mi)(fix|close|resolve|fixes|closes|resolves|fixed|closed|resolved)[\s]*.*(((and )?(#|https:\/\/github.com\/%s\/issues\/)\d+[ ]*)+)"
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  prBodyClosePattern: e.target.value,
                })
              }
              rows={2}
            />
          </Form.Item>
        </>
      ),
    },
    {
      key: 'ADDITIONAL',
      label: 'Additional Settings',
      style: panelStyle,
      children: (
        <>
          <p>
            Enable the <ExternalLink link={DOC_URL.PLUGIN.REFDIFF}>RefDiff</ExternalLink> plugin to pre-calculate
            version-based metrics
            <HelpTooltip content="Calculate the commits diff between two consecutive tags that match the following RegEx. Issues closed by PRs which contain these commits will also be calculated. The result will be shown in table.refs_commits_diffs and table.refs_issues_diffs." />
          </p>
          <div className="refdiff">
            Compare the last
            <Input
              style={{ margin: '0 8px', width: 60 }}
              placeholder="10"
              value={transformation.refdiff?.tagsLimit ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  refdiff: {
                    ...transformation?.refdiff,
                    tagsLimit: +e.target.value,
                  },
                })
              }
            />
            tags that match the
            <Input
              style={{ margin: '0 8px', width: 200 }}
              placeholder="(regex)$"
              value={transformation.refdiff?.tagsPattern ?? ''}
              onChange={(e) =>
                onChangeTransformation({
                  ...transformation,
                  refdiff: {
                    ...transformation?.refdiff,
                    tagsPattern: e.target.value,
                  },
                })
              }
            />
            for calculation
          </div>
          <h3 style={{ marginBottom: 16, marginTop: 16 }}>
            <span>PR Size Exclusions</span>
          </h3>
          <div style={{ margin: '8px 0' }}>
            <span>Exclude file extensions (comma-separated, e.g. .md,.txt,.json)</span>
            <Input
              style={{ width: 360, margin: '0 8px' }}
              placeholder=".md,.txt,.json"
              value={(transformation.prSizeExcludedFileExtensions || []).join(',')}
              onChange={(e) => {
                // Don't filter during onChange to allow typing commas freely
                const extensions = e.target.value.split(',').map((s: string) => s.trim());
                onChangeTransformation({
                  ...transformation,
                  prSizeExcludedFileExtensions: extensions,
                });
              }}
              onBlur={(e) => {
                // Clean up empty entries when user leaves the field
                const extensions = e.target.value
                  .split(',')
                  .map((s: string) => s.trim())
                  .filter((s: string) => s.length > 0);
                onChangeTransformation({
                  ...transformation,
                  prSizeExcludedFileExtensions: extensions,
                });
              }}
            />
            <HelpTooltip content="These extensions are ignored when computing PR Size (additions/deletions)." />
          </div>
        </>
      ),
    },
  ].filter((it) => entities.includes(it.key) || it.key === 'ADDITIONAL');
