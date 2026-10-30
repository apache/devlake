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

package plugin

import (
	"fmt"
	"strings"
	"sync"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
)

// Allowing plugin to know each other

var (
	plugins     map[string]PluginMeta
	pluginMutex sync.RWMutex
)

func RegisterPlugin(name string, plugin PluginMeta) errors.Error {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()
	if plugins == nil {
		plugins = make(map[string]PluginMeta)
	}
	plugins[name] = plugin
	return nil
}

// MustRegister registers p under the name it reports, panicking if the plugin
// cannot be identified.
//
// The generated plugins/all package calls it for every plugin that is compiled
// into the binary, which makes the plugin set a build-time decision. Registering
// a name twice overwrites the previous entry, so a plugin that is both compiled
// in and loaded from a shared object keeps working: the loader registers the
// same value again.
func MustRegister(p PluginMeta) {
	if p == nil {
		panic("plugin: MustRegister called with a nil plugin")
	}
	name := p.Name()
	if name == "" {
		panic("plugin: MustRegister called with an empty plugin name")
	}
	if err := RegisterPlugin(name, p); err != nil {
		panic(err)
	}
}

func GetPlugin(name string) (PluginMeta, errors.Error) {
	if plugins == nil {
		return nil, errors.Default.New("RegisterPlugin have never been called.")
	}
	if plugin, ok := plugins[name]; ok {
		return plugin, nil
	}
	return nil, errors.Default.New(fmt.Sprintf("Plugin `%s` doesn't exist", name))
}

type PluginCallBack func(name string, plugin PluginMeta) errors.Error

func TraversalPlugin(handle PluginCallBack) errors.Error {
	for name, plugin := range plugins {
		err := handle(name, plugin)
		if err != nil {
			return err
		}
	}
	return nil
}

func AllPlugins() map[string]PluginMeta {
	return plugins
}

func FindPluginNameBySubPkgPath(subPkgPath string) (string, errors.Error) {
	for name, plugin := range plugins {
		if strings.HasPrefix(subPkgPath, plugin.RootPkgPath()) {
			return name, nil
		}
	}
	return "", errors.Default.New(fmt.Sprintf("Unable to find plugin for subPkgPath %s", subPkgPath))
}

func InitPlugins(basicRes context.BasicRes) {
	for pluginName, pluginMeta := range plugins {
		if pluginEntry, ok := pluginMeta.(PluginInit); ok {
			err := pluginEntry.Init(basicRes)
			if err != nil {
				panic(fmt.Errorf("failed to initialize plugin %v due to %w", pluginName, err))
			}
		} else {
			basicRes.GetLogger().Info("plugin: %s doesn't implement 'PluginInit', it will be skipped.", pluginName)
		}
	}
}
