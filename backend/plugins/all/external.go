/*
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
*/

//go:build devlake_external_plugins

// Package all is empty in a core-only build.
//
// A core-only binary compiles in the framework and the API but no plugin, which
// is useful for a minimal image: every plugin is then sideloaded as a shared
// object from PLUGIN_DIR. The default build links the official plugins in and
// still accepts extra .so files from PLUGIN_DIR on top.
//
// A sideloaded plugin must be built with the same Go toolchain, build flags and
// dependency versions as the binary loading it, so build it in the matching
// builder image rather than on the host.
package all
