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

import { Form, Input } from 'antd';

import { Block } from '@/components';

interface Props {
  entities: string[];
  transformation: any;
  setTransformation: React.Dispatch<React.SetStateAction<any>>;
}

export const ZephyrTransformation = ({ entities, transformation, setTransformation }: Props) => {
  if (!entities.includes('CODEQUALITY')) {
    return null;
  }

  return (
    <Block
      title="Test case mapping"
      description="Configure how Zephyr custom fields map into DevLake QA domain models."
    >
      <Form labelCol={{ span: 8 }}>
        <Form.Item label="Test type custom field">
          <Input
            value={transformation.testTypeCustomField ?? 'Test Type'}
            placeholder="Test Type"
            onChange={(e) =>
              setTransformation({
                ...transformation,
                testTypeCustomField: e.target.value,
              })
            }
          />
        </Form.Item>
      </Form>
    </Block>
  );
};
