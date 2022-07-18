package cmd

import (
	"embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/haveachin/infrared/internal/plugin/prometheus"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/haveachin/infrared/internal/plugin/webhook"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	files embed.FS

	configPath  string
	workingDir  string
	environment string

	logger *zap.Logger

	mu            sync.Mutex
	proxies       []*infrared.Proxy
	pluginManager infrared.PluginManager

	rootCmd = &cobra.Command{
		Use:   "infrared",
		Short: "Starts the infrared proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			logger, err = newLogger()
			if err != nil {
				return err
			}
			defer logger.Sync()

			if err := os.Chdir(workingDir); err != nil {
				return err
			}

			logger.Info("loading proxy from config",
				zap.String("config", configPath),
			)

			if err := safeWriteFromEmbeddedFS("configs", "."); err != nil {
				return err
			}

			cfg, err := config.New(configPath, onConfigChange, logger)
			if err != nil {
				return err
			}

			data, err := cfg.Read()
			if err != nil {
				return err
			}

			v := viper.New()
			v.MergeConfigMap(data)
			prxCfgs := []infrared.ProxyConfig{
				java.ProxyConfig{Viper: v},
				bedrock.ProxyConfig{Viper: v},
			}

			pluginManager = infrared.PluginManager{
				Plugins: []infrared.Plugin{
					&webhook.Plugin{},
					&prometheus.Plugin{},
				},
				Logger: logger,
			}
			logger.Info("loading plugins")
			pluginManager.LoadPlugins(data)
			logger.Info("enabling plugins")
			pluginManager.EnablePlugins()
			defer pluginManager.DisablePlugins()

			logger.Info("starting proxies")
			for _, prxCfg := range prxCfgs {
				p, err := infrared.NewProxy(prxCfg)
				if err != nil {
					return err
				}
				proxies = append(proxies, p)
				go p.ListenAndServe(logger)
			}

			sc := make(chan os.Signal, 1)
			signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
			<-sc
			return nil
		},
	}
)

func init() {
	v := viper.New()
	v.SetEnvPrefix("INFRARED")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	rootCmd.Flags().StringVarP(&configPath, "config", "c", "config.yml", "path of the config file")
	rootCmd.Flags().StringVarP(&workingDir, "working-dir", "w", ".", "set the working directory")
	rootCmd.Flags().StringVarP(&environment, "environment", "e", "prod", "set the deployment environment")
	viper.BindPFlag("CONFIG", rootCmd.Flags().Lookup("config"))

	rootCmd.AddCommand(licenseCmd)
}

func newLogger() (*zap.Logger, error) {
	var logger *zap.Logger
	var err error
	switch environment {
	case "dev":
		logger, err = zap.NewDevelopment()
	case "prod":
		logger, err = zap.NewProduction()
	default:
		return nil, fmt.Errorf("unsupported environment %q", environment)
	}
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// Execute executes the root command.
func Execute(fs embed.FS) error {
	files = fs
	return rootCmd.Execute()
}

func safeWriteFromEmbeddedFS(embedPath, sysPath string) error {
	entries, err := files.ReadDir(embedPath)
	if err != nil {
		return err
	}

	for _, e := range entries {
		ePath := filepath.Join(embedPath, e.Name())
		sPath := filepath.Join(sysPath, e.Name())

		if _, err := os.Stat(sPath); err == nil || !os.IsNotExist(err) {
			continue
		}

		if e.IsDir() {
			if err := os.MkdirAll(sPath, 0755); err != nil {
				return err
			}

			safeWriteFromEmbeddedFS(ePath, sPath)
			continue
		}

		bb, err := files.ReadFile(ePath)
		if err != nil {
			return err
		}

		if err := os.WriteFile(sPath, bb, 0755); err != nil {
			return err
		}
	}

	return nil
}

func onConfigChange(cfg map[string]interface{}) {
	mu.Lock()
	defer mu.Unlock()

	v := viper.New()
	v.MergeConfigMap(cfg)

	cfgs := []infrared.ProxyConfig{
		java.ProxyConfig{Viper: v},
		bedrock.ProxyConfig{Viper: v},
	}

	logger.Info("Reloading proxies")
	for n, p := range proxies {
		if err := p.Reload(cfgs[n]); err != nil {
			logger.Error("failed to reload proxy",
				zap.Error(err),
			)
		}
	}

	logger.Info("Reloading plugins")
	pluginManager.ReloadPlugins(cfg)
}
