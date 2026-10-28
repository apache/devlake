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
import { Checkbox, Collapse, Divider, Form, Input, InputNumber, message, Modal, Select, Space, Spin, Typography } from 'antd';

import API from '@/api';
import { IAiReviewScopeConfig } from '@/api/plugin/aireview';

const { Text } = Typography;

interface Props {
  scopeConfigId?: number;
  onCancel: () => void;
  onSave: (id: number) => void;
}

interface ToolSectionProps {
  label: string;
  enabledField: string;
  usernameField: string;
  patternField: string;
  enabled: boolean;
}

const ToolSection = ({ label, enabledField, usernameField, patternField, enabled }: ToolSectionProps) => (
  <Space direction="vertical" style={{ width: '100%' }}>
    <Form.Item name={enabledField} valuePropName="checked" style={{ marginBottom: 8 }}>
      <Checkbox>{`Enable ${label}`}</Checkbox>
    </Form.Item>
    {enabled && (
      <>
        <Form.Item
          label="Bot Username"
          name={usernameField}
          tooltip="The GitHub username the bot posts review comments as"
          style={{ marginBottom: 8 }}
        >
          <Input placeholder={`e.g. ${label.toLowerCase().replace(/\s/g, '')}-bot`} />
        </Form.Item>
        <Form.Item
          label="Detection Pattern (regex)"
          name={patternField}
          tooltip="Regex matched against the review comment body to confirm this tool wrote it"
          style={{ marginBottom: 0 }}
        >
          <Input placeholder="(?i)(pattern)" />
        </Form.Item>
      </>
    )}
  </Space>
);

export const AiReviewScopeConfigModal = ({ scopeConfigId, onCancel, onSave }: Props) => {
  const [form] = Form.useForm<IAiReviewScopeConfig>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  // Track enabled toggles for conditional field display
  const codeRabbitEnabled = Form.useWatch('codeRabbitEnabled', form);
  const cursorBugbotEnabled = Form.useWatch('cursorBugbotEnabled', form);
  const qodoEnabled = Form.useWatch('qodoEnabled', form);
  const geminiEnabled = Form.useWatch('geminiEnabled', form);

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      try {
        const config = scopeConfigId
          ? await API.plugin.aireview.getScopeConfig(scopeConfigId)
          : await API.plugin.aireview.getDefaultScopeConfig();
        if (config) {
          form.setFieldsValue(config);
        }
      } catch {
        message.error('Failed to load configuration.');
      } finally {
        setLoading(false);
      }
    };
    load();
  }, [scopeConfigId, form]);

  const handleSave = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const saved = scopeConfigId
        ? await API.plugin.aireview.updateScopeConfig(scopeConfigId, values)
        : await API.plugin.aireview.createScopeConfig(values);
      if (saved.id == null) {
        message.error('Saved configuration is missing an ID.');
        return;
      }
      onSave(saved.id);
    } catch {
      message.error('Failed to save configuration. Please try again.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open
      width={720}
      title="AI Review Configuration"
      okText="Save"
      confirmLoading={saving}
      onCancel={onCancel}
      onOk={handleSave}
    >
      <Spin spinning={loading}>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* ── Prediction Thresholds ── */}
          <Divider orientation="left" plain>
            Prediction Thresholds
          </Divider>
          <Space size="large" wrap>
            <Form.Item
              label="Warning Threshold (risk score)"
              name="warningThreshold"
              tooltip="PRs with a risk score at or above this value are flagged as risky. Used to classify TP/FP/FN/TN against actual CI outcomes."
              style={{ marginBottom: 16 }}
            >
              <InputNumber min={0} max={100} style={{ width: 120 }} />
            </Form.Item>
            <Form.Item
              label="Observation Window (days)"
              name="observationWindowDays"
              tooltip="Number of days after merge to observe for post-merge failures"
              style={{ marginBottom: 16 }}
            >
              <InputNumber min={1} max={365} style={{ width: 120 }} />
            </Form.Item>
            <Form.Item
              label="CI Failure Source"
              name="ciFailureSource"
              tooltip="Which CI data to use when determining if a PR actually failed. 'Test Cases' is more accurate (quarantines flaky tests) but requires full testregistry artifact collection. 'Job Result' works without artifact collection. 'Both' computes predictions for each source separately."
              style={{ marginBottom: 16 }}
            >
              <Select style={{ width: 280 }}>
                <Select.Option value="both">Both — compute predictions for each source</Select.Option>
                <Select.Option value="job_result">Job Result — fast, no artifact collection needed</Select.Option>
                <Select.Option value="test_cases">Test Cases — accurate, requires artifact collection</Select.Option>
              </Select>
            </Form.Item>
          </Space>

          {/* ── CI Failure Filtering ── */}
          <Divider orientation="left" plain>
            CI Failure Filtering
          </Divider>
          <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            Filter out noisy CI failures to improve prediction accuracy. Changes take effect on the next blueprint run.
          </Text>
          <Form.Item name="excludeFlakyTests" valuePropName="checked" style={{ marginBottom: 8 }}>
            <Checkbox>
              Exclude flaky tests — ignore tests/jobs that also fail on periodic or push runs (30-day lookback window)
            </Checkbox>
          </Form.Item>
          <Form.Item name="excludeInfraFailures" valuePropName="checked" style={{ marginBottom: 16 }}>
            <Checkbox>
              Exclude infrastructure failures — ignore job failures where no test cases actually failed (e.g. cluster
              provisioning, quota, timeout). Requires test case data to be collected (CI source &quot;Test Cases&quot; or
              &quot;Both&quot;).
            </Checkbox>
          </Form.Item>

          {/* ── CI Backfill ── */}
          <Divider orientation="left" plain>
            CI Data Backfill (Openshift CI / GCS)
          </Divider>
          <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            Fetch historical CI job results from the public Openshift CI GCS bucket for PRs that have AI reviews but no
            CI data yet. Requires network access to GCS. Set to 0 to disable backfill.
          </Text>
          <Form.Item
            label="Backfill window (days)"
            name="ciBackfillDays"
            tooltip="How many days back to look for missing CI data. Set to 0 to disable."
            style={{ marginBottom: 16 }}
          >
            <InputNumber min={0} max={3650} style={{ width: 120 }} />
          </Form.Item>

          {/* ── Tool Detection ── */}
          <Divider orientation="left" plain>
            AI Tool Detection
          </Divider>
          <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            Configure which AI review bots to detect. Each tool is identified by its bot username and a regex pattern
            matched against comment bodies.
          </Text>
          <Collapse
            defaultActiveKey={['coderabbit', 'qodo']}
            style={{ marginBottom: 16 }}
            items={[
              {
                key: 'coderabbit',
                label: 'CodeRabbit',
                children: (
                  <ToolSection
                    label="CodeRabbit"
                    enabledField="codeRabbitEnabled"
                    usernameField="codeRabbitUsername"
                    patternField="codeRabbitPattern"
                    enabled={!!codeRabbitEnabled}
                  />
                ),
              },
              {
                key: 'qodo',
                label: 'Qodo (formerly Codium)',
                children: (
                  <ToolSection
                    label="Qodo"
                    enabledField="qodoEnabled"
                    usernameField="qodoUsername"
                    patternField="qodoPattern"
                    enabled={!!qodoEnabled}
                  />
                ),
              },
              {
                key: 'gemini',
                label: 'Gemini Code Assist',
                children: (
                  <ToolSection
                    label="Gemini Code Assist"
                    enabledField="geminiEnabled"
                    usernameField="geminiUsername"
                    patternField="geminiPattern"
                    enabled={!!geminiEnabled}
                  />
                ),
              },
              {
                key: 'cursor',
                label: 'Cursor Bugbot',
                children: (
                  <ToolSection
                    label="Cursor Bugbot"
                    enabledField="cursorBugbotEnabled"
                    usernameField="cursorBugbotUsername"
                    patternField="cursorBugbotPattern"
                    enabled={!!cursorBugbotEnabled}
                  />
                ),
              },
            ]}
          />

          {/* ── Advanced Settings - Matching Patterns ── */}
          <Collapse
            style={{ marginBottom: 8 }}
            items={[
              {
                key: 'advanced',
                label: 'Advanced Settings — Matching Patterns',
                children: (
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
                      Regex patterns for risk level classification and AI detection.
                    </Text>
                    <Form.Item
                      label="High Risk Pattern"
                      name="riskHighPattern"
                      tooltip="Regex to identify high-risk review comments"
                      style={{ marginBottom: 12 }}
                    >
                      <Input placeholder="(?i)(critical|security|breaking|major)" />
                    </Form.Item>
                    <Form.Item
                      label="Medium Risk Pattern"
                      name="riskMediumPattern"
                      tooltip="Regex to identify medium-risk review comments"
                      style={{ marginBottom: 12 }}
                    >
                      <Input placeholder="(?i)(warning|medium|moderate)" />
                    </Form.Item>
                    <Form.Item
                      label="Low Risk Pattern"
                      name="riskLowPattern"
                      tooltip="Regex to identify low-risk review comments"
                      style={{ marginBottom: 12 }}
                    >
                      <Input placeholder="(?i)(minor|low|info|suggestion)" />
                    </Form.Item>
                    <Form.Item
                      label="AI Commit Patterns"
                      name="aiCommitPatterns"
                      tooltip="Optional extra regex matched against commit-message trailers after built-in Co-Authored-By / Assisted-by / Made-with detection. A single regex (do not comma-split)."
                      style={{ marginBottom: 12 }}
                    >
                      <Input placeholder="(?i)(?:co-authored-by|assisted-by|made-with):.*(?:\bai\b|chatgpt|openai)" />
                    </Form.Item>
                    <Form.Item
                      label="AI PR Label Pattern"
                      name="aiPrLabelPattern"
                      tooltip="Regex matched against PR labels to detect AI-reviewed PRs"
                      style={{ marginBottom: 12 }}
                    >
                      <Input placeholder="(?i)(ai-reviewed|coderabbit|automated-review)" />
                    </Form.Item>
                    <Form.Item
                      label="Bug Link Pattern"
                      name="bugLinkPattern"
                      tooltip="Regex to extract issue links from PR descriptions for post-merge bug tracking"
                      style={{ marginBottom: 0 }}
                    >
                      <Input placeholder="(?i)(fixes|closes|resolves)\s*#(\d+)" />
                    </Form.Item>
                  </Space>
                ),
              },
            ]}
          />
        </Form>
      </Spin>
    </Modal>
  );
};
