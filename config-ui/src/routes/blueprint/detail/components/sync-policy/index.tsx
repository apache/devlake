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

import { useState, useEffect, useMemo } from 'react';
import dayjs from 'dayjs';
import type { RadioChangeEvent } from 'antd';
import { Radio, Space, Checkbox, Input, Tag, DatePicker } from 'antd';

import { Block, ExternalLink } from '@/components';
import { getCron, getCronOptions } from '@/config';
import { formatTime } from '@/utils';

import * as S from './styled';

interface Props {
  isManual: boolean;
  cronConfig: string;
  skipOnFail: boolean;
  showTimeFilter: boolean;
  timeAfter: string | null;
  onChangeIsManual: (val: boolean) => void;
  onChangeCronConfig: (val: string) => void;
  onChangeSkipOnFail: (val: boolean) => void;
  onChangeTimeAfter: (val: string | null) => void;
}

export const SyncPolicy = ({
  isManual,
  cronConfig,
  skipOnFail,
  showTimeFilter,
  timeAfter,
  onChangeIsManual,
  onChangeCronConfig,
  onChangeSkipOnFail,
  onChangeTimeAfter,
}: Props) => {
  const [selectedValue, setSelectedValue] = useState('Daily');

  const cronOpts = getCronOptions();

  useEffect(() => {
    if (isManual) {
      setSelectedValue('Manual');
    } else {
      const opt = cronOpts.find((it) => it.value === cronConfig);
      setSelectedValue(opt ? opt.label : 'Custom');
    }
  }, [isManual, cronConfig]);

  const [timezone, quickTimeOpts] = useMemo(() => {
    const timezone = dayjs().format('ZZ').replace('00', '');
    const quickTimeOpts = [
      { label: 'Last 6 months', date: dayjs().subtract(6, 'month').toDate() },
      { label: 'Last 90 days', date: dayjs().subtract(90, 'day').toDate() },
      { label: 'Last 30 days', date: dayjs().subtract(30, 'day').toDate() },
      { label: 'Last Year', date: dayjs().subtract(1, 'year').toDate() },
    ];
    return [timezone, quickTimeOpts];
  }, []);

  const cron = useMemo(() => getCron(isManual, cronConfig), [isManual, cronConfig]);

  const [minute, hour, day, month, week] = useMemo(() => cronConfig.split(' '), [cronConfig]);

  const handleChangeFrequency = (e: RadioChangeEvent) => {
    const value = e.target.value;
    setSelectedValue(value);
    if (value === 'Manual') {
      onChangeIsManual(true);
    } else if (value === 'Custom') {
      onChangeIsManual(false);
      onChangeCronConfig('* * * * *');
    } else {
      const opt = cronOpts.find((it) => it.label === value) as any;
      onChangeIsManual(false);
      onChangeCronConfig(opt.value);
    }
  };

  return (
    <S.Wrapper>
      <div className="timezone">
        Your local time zone is <strong>UTC {timezone}</strong>. All time listed below is shown in your local time.
      </div>
      {showTimeFilter && (
        <Block
          title="Time Range"
          description="Select the time range for the data you wish to collect. DevLake will collect the last six months of data by default."
        >
          <div className="quick-selection">
            {quickTimeOpts.map((opt, i) => (
              <Tag
                key={i}
                style={{ marginRight: 5, cursor: 'pointer' }}
                color={formatTime(opt.date) === formatTime(timeAfter) ? 'blue' : 'default'}
                onClick={() => onChangeTimeAfter(dayjs(opt.date).utc().format('YYYY-MM-DD[T]HH:mm:ssZ'))}
              >
                {opt.label}
              </Tag>
            ))}
          </div>

          <div className="time-selection">
            <DatePicker
              value={timeAfter ? dayjs(timeAfter) : null}
              placeholder="Select start from"
              onChange={(_, date) =>
                onChangeTimeAfter(
                  date
                    ? dayjs(date as string)
                        .utc()
                        .format('YYYY-MM-DD[T]HH:mm:ssZ')
                    : null,
                )
              }
            />
            <strong>to Now</strong>
          </div>
        </Block>
      )}
      <div className="cron">
        <Block
          style={{ flex: '0 0 450px', marginRight: 20 }}
          title="Sync Frequency"
          description="Blueprints will run on creation and recurringly based on the schedule."
        >
          <Radio.Group value={selectedValue} onChange={handleChangeFrequency}>
            <Space direction="vertical">
              {cronOpts.map(({ label, subLabel }) => (
                <Radio key={label} value={label}>{`${label} ${subLabel}`}</Radio>
              ))}
            </Space>
          </Radio.Group>
          {selectedValue === 'Custom' && (
            <>
              <Space>
                <Block title="Minute">
                  <Input
                    value={minute}
                    onChange={(e) => onChangeCronConfig([e.target.value, hour, day, month, week].join(' '))}
                  />
                </Block>
                <Block title="Hour">
                  <Input
                    value={hour}
                    onChange={(e) => onChangeCronConfig([minute, e.target.value, day, month, week].join(' '))}
                  />
                </Block>
                <Block title="Day">
                  <Input
                    value={day}
                    onChange={(e) => onChangeCronConfig([minute, hour, e.target.value, month, week].join(' '))}
                  />
                </Block>
                <Block title="Month">
                  <Input
                    value={month}
                    onChange={(e) => onChangeCronConfig([minute, hour, day, e.target.value, week].join(' '))}
                  />
                </Block>
                <Block title="Week">
                  <Input
                    value={week}
                    onChange={(e) => onChangeCronConfig([minute, hour, day, month, e.target.value].join(' '))}
                  />
                </Block>
              </Space>
              {!cron.nextTime && <S.Error>Invalid Cron code, please enter again.</S.Error>}
            </>
          )}
          <div style={{ marginTop: 16 }}>
            <ExternalLink link="https://crontab.cronhub.io/">Learn how to use cron code</ExternalLink> or{' '}
            <ExternalLink link="https://cron-ai.vercel.app/">auto-convert English to cron code</ExternalLink>
          </div>
        </Block>
        <Block title="Next Three Runs:">
          {cron.nextTimes.length ? (
            <ul>
              {cron.nextTimes.map((it, i) => (
                <li key={i}>
                  {dayjs(it).format('YYYY-MM-DD HH:mm A')}({dayjs(it).fromNow()})
                </li>
              ))}
            </ul>
          ) : (
            'N/A'
          )}
        </Block>
      </div>
      <Block title="Running Policy">
        <Checkbox checked={skipOnFail} onChange={(e) => onChangeSkipOnFail(e.target.checked)}>
          Skip failed tasks (Recommended when collecting a large volume of data, eg. 10+ GitHub repos, Jira boards,
          etc.)
        </Checkbox>
        <p style={{ paddingLeft: 28 }}>
          A task is a unit of a pipeline, an execution of a blueprint. By default, when a task is failed, the whole
          pipeline will fail and all the data that has been collected will be discarded. By skipping failed tasks, the
          pipeline will continue to run, and the data collected by successful tasks will not be affected. After the
          pipeline is finished, you can rerun these failed tasks.
        </p>
      </Block>
    </S.Wrapper>
  );
};
