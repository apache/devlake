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
import { Select, Button } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import API from '@/api';
import { Block } from '@/components';

interface Props {
  initialValue: string;
  value: string;
  error: string;
  setValue: (value: string) => void;
  setError: (error: string) => void;
}

interface IProject {
  name: string;
  description?: string;
}

export const ProjectSelect = ({ initialValue, value, setValue, setError }: Props) => {
  const [projects, setProjects] = useState<IProject[]>([]);
  const [loading, setLoading] = useState(false);

  // Initialize with initialValue (like DBUrl component does)
  // Only sync if value is empty or different from initialValue
  useEffect(() => {
    if (initialValue && (!value || value !== initialValue)) {
      setValue(initialValue);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialValue]);

  // Fetch DevLake projects
  const fetchProjects = async () => {
    setLoading(true);
    try {
      const response = await API.project.list({ pageSize: 1000, page: 1 });
      setProjects(response.projects || []);
    } catch {
      setProjects([]);
    } finally {
      setLoading(false);
    }
  };

  // Fetch projects on mount
  useEffect(() => {
    fetchProjects();
  }, []);

  // Validate selection (only if value exists or was changed by user)
  useEffect(() => {
    // Skip validation on initial mount if value is empty (allows form to start without errors)
    if (value === '' && !initialValue) {
      return;
    }
    
    if (!value) {
      setError('Project is required');
    } else {
      setError('');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const handleChange = (selectedValue: string) => {
    setValue(selectedValue);
  };

  const options = projects.map((project: IProject) => ({
    label: project.name,
    value: project.name,
  }));

  return (
    <Block
      title="DevLake Project"
      description="Select an existing DevLake project to associate with this connection"
      required
    >
      <Select
        style={{ width: 386 }}
        placeholder={loading ? "Loading projects..." : "Select a DevLake project..."}
        value={value || undefined}
        onChange={handleChange}
        options={options}
        loading={loading}
        disabled={loading}
        showSearch
        filterOption={(input: string, option?: { label?: string }) =>
          (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
        }
        notFoundContent={loading ? 'Loading projects...' : projects.length === 0 ? 'No DevLake projects found. Create a project first.' : undefined}
      />
      <Button
        type="link"
        icon={<ReloadOutlined />}
        onClick={fetchProjects}
        loading={loading}
        style={{ marginTop: 8 }}
      >
        Refresh Projects
      </Button>
    </Block>
  );
};

