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

package runner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	goplugin "plugin"
	"strings"
	"sync"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/server/services/remote"
	"github.com/apache/devlake/server/services/remote/bridge"
	"github.com/apache/devlake/server/services/remote/models"
)

// LoadPlugins load plugins from local directory
func LoadPlugins(basicRes context.BasicRes) errors.Error {
	err := LoadGoPlugins(basicRes)
	if err != nil {
		return err
	}
	if !basicRes.GetConfigReader().GetBool("DISABLED_REMOTE_PLUGINS") {
		err = LoadRemotePlugins(basicRes)
		if err != nil {
			return err
		}
	}
	return nil
}

func LoadGoPlugins(basicRes context.BasicRes) errors.Error {
	pluginsDir := basicRes.GetConfig("PLUGIN_DIR")
	if pluginsDir == "" {
		return nil
	}
	// The official binary already has every official plugin compiled in, so a
	// missing directory simply means "nothing to sideload".
	if _, statErr := os.Stat(pluginsDir); os.IsNotExist(statErr) {
		basicRes.GetLogger().Info("plugin directory %s does not exist, no plugins to sideload", pluginsDir)
		return nil
	}

	var wg sync.WaitGroup
	walkErr := filepath.WalkDir(pluginsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fileName := d.Name()
		if !strings.HasSuffix(fileName, ".so") || fileName == ".so" {
			return nil
		}

		pluginName := strings.TrimSuffix(fileName, ".so")
		pluginMeta, loadErr := openGoPlugin(path)
		if loadErr != nil {
			// Sideloaded plugins are third-party code built outside this repo. One
			// incompatible or broken file must not keep the server from starting,
			// so it is skipped loudly instead. The usual cause is a toolchain or
			// dependency mismatch with the binary that is loading it.
			basicRes.GetLogger().Error(loadErr, "skipping plugin %s from %s", pluginName, path)
			return nil
		}
		if _, err := plugin.GetPlugin(pluginName); err == nil {
			basicRes.GetLogger().Warn(nil, "plugin %s is already registered, the shared object %s takes precedence", pluginName, path)
		}

		wg.Add(1)
		go func(name string, meta plugin.PluginMeta) {
			defer wg.Done()
			if err := plugin.RegisterPlugin(name, meta); err != nil {
				panic(err)
			}
			basicRes.GetLogger().Info("plugin loaded %s", name)
		}(pluginName, pluginMeta)
		return nil
	})
	wg.Wait()
	return errors.Convert(walkErr)
}

// openGoPlugin opens a shared object and reads its exported entry point.
func openGoPlugin(path string) (plugin.PluginMeta, error) {
	plug, err := goplugin.Open(path)
	if err != nil {
		return nil, err
	}
	symPluginEntry, err := plug.Lookup("PluginEntry")
	if err != nil {
		return nil, err
	}
	pluginMeta, ok := symPluginEntry.(plugin.PluginMeta)
	if !ok {
		return nil, fmt.Errorf("%s: PluginEntry must implement plugin.PluginMeta", filepath.Base(path))
	}
	return pluginMeta, nil
}

func LoadRemotePlugins(basicRes context.BasicRes) errors.Error {
	remotePluginDir := basicRes.GetConfig("REMOTE_PLUGIN_DIR")
	if remotePluginDir != "" {
		basicRes.GetLogger().Info("Loading remote plugins")
		remote.Init(basicRes)
		walkErr := filepath.WalkDir(remotePluginDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			fileName := d.Name()
			if fileName == "run.sh" {
				invoker := bridge.NewCmdInvoker(path)
				result := invoker.Call("plugin-info", bridge.DefaultContext)
				if result.Err != nil {
					return errors.Default.Wrap(result.Err, "Error calling plugin-info")
				}
				pluginInfo := &models.PluginInfo{}
				err := result.Get(pluginInfo)
				if err != nil {
					return err
				}
				remotePlugin, err := remote.NewRemotePlugin(pluginInfo)
				if err != nil {
					return err
				}
				err = plugin.RegisterPlugin(pluginInfo.Name, remotePlugin)
				if err != nil {
					return err
				}
				basicRes.GetLogger().Info(`remote plugin loaded %s`, pluginInfo.Name)
			}
			return nil
		})
		return errors.Convert(walkErr)
	}
	return nil
}
