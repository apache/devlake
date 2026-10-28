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
# lake-builder

Golang builder image for DevLake. Used by GitHub workflows.

Published as `apache/devlake:lake-builder-v#.#.#`
(https://hub.docker.com/r/apache/devlake/tags?name=lake-builder).
Earlier versions (up to v0.0.15) were published as `mericodev/lake-builder`.

## Tagged Release
1. Create a tag matching the pattern `builder-v#.#.#`, e.g. `builder-v0.0.16`. Determine the previous tag first so you version
it correctly. Example command: `git tag builder-v0.0.16`
2. Push the tag to origin. Example command: `git push origin builder-v0.0.16`
3. The `Build-BuilderImage-Push-Docker` workflow (`.github/workflows/build-builder.yml`) builds this directory and pushes
`apache/devlake:lake-builder-v0.0.16`.
4. Point the `container:` lines in `.github/workflows/` to the new tag in a follow-up PR.
