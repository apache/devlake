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
## Push API

## Summary

This is a generic API service that gives our users the ability to inject data directly to their own database using a
simple, all-purpose endpoint.

The push API is disabled by default. To enable it safely, you must:
- configure `PUSH_API_ALLOWED_TABLES` with the specific non-internal tables that may be written
- send a Bearer API key whose `allowedPath` matches the `/push/...` endpoint you are calling

## The Endpoint

POST to ```localhost:8080/push/:tableName```

Where "tableName" is the name of the table you wish to insert into
For example, "commits" would be ```/push/commits```

Internal `_devlake_*` tables are never writable through this endpoint, even if
they are listed in `PUSH_API_ALLOWED_TABLES`.

## The JSON body

Include a JSON body that consists of an array of objects you wish to insert.
Please Note: You must know the schema you are inserting into (column names, types, etc.)

```
[
    {
        "id": "gitlab...etc",
        "sha": "osidjfoawehfwh08",
        "additions": 89,
        ...
    }
]
```


