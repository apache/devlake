<!--
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->


# CheckmarxOne Plugin

## Summary

This plugin collects security findings and vulnerabilities from [CheckmarxOne](https://checkmarx.com/checkmarx-one/) - a leading application security testing platform.

## Features

- Collect security findings/vulnerabilities from CheckmarxOne projects
- Track vulnerability severity, status, and remediation progress
- Support for multiple projects
- Integration with DevLake's security domain layer

## Requirements

- CheckmarxOne account and API access
- Server URL, Client ID, and Client Secret from CheckmarxOne

## Configuration

### Connection Setup

Create a connection to CheckmarxOne using the following fields:

- **Server URL**: The base URL of your CheckmarxOne instance (e.g., `https://checkmarx.mycompany.com`)
- **Client ID**: OAuth client ID for API access
- **Client Secret**: OAuth client secret for API access
- **Username**: (Optional) Username for authentication
- **Password**: (Optional) Password for authentication

### Scope Configuration

Select the CheckmarxOne projects you want to collect data from:

- **Project ID**: The unique identifier of the CheckmarxOne project

## Data Collection

The plugin collects the following data:

### Findings
- Finding ID and Name
- Severity Level (Critical, High, Medium, Low)
- Status (Open, Fixed, Suppressed)
- First Found and Last Found timestamps
- Finding Description
- Type of finding

## API Reference

### POST /connections
Create a new CheckmarxOne connection

### GET /connections
List all CheckmarxOne connections

### GET /connections/:connectionId
Get details of a specific connection

### PATCH /connections/:connectionId
Update a CheckmarxOne connection

### DELETE /connections/:connectionId
Delete a CheckmarxOne connection

## Troubleshooting

### Authentication Issues
- Verify Client ID and Client Secret are correct
- Ensure the API user has appropriate permissions in CheckmarxOne
- Check that the Server URL is accessible from the DevLake instance

### No Data Collected
- Verify that the project ID exists in CheckmarxOne
- Check that the API client has access to the specified project
- Review the logs for any API errors

## Development

To build and test the plugin locally:

```bash
cd backend/plugins/checkmarxone
go build
```

For standalone debugging:

```bash
./checkmarxone --connectionId=1 --projectId=myproject
```
