package cmd

import (
	"embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/haveachin/infrared/internal/plugin/api"
	"github.com/haveachin/infrared/internal/plugin/prometheus"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/bedrock"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/internal/pkg/java"
	"github.com/haveachin/infrared/internal/plugin/webhook"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	files embed.FS

	configPath  string
	workingDir  string
	environment string

	logger        *zap.Logger
	pluginManager infrared.PluginManager

	mu      sync.Mutex
	proxies = map[infrared.Edition]*infrared.Proxy{}

	rootCmd = &cobra.Command{
		Use:   "infrared",
		Short: "Starts the infrared proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := initLogger(); err != nil {
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

			javaPrxCfg, err := java.NewProxyConfigFromMap(data)
			if err != nil {
				return err
			}

			javaPrx, err := infrared.NewProxy(javaPrxCfg)
			if err != nil {
				return err
			}
			proxies[infrared.JavaEdition] = javaPrx

			bedrockPrxCfg, err := bedrock.NewProxyConfigFromMap(data)
			if err != nil {
				return err
			}

			bedrockPrx, err := infrared.NewProxy(bedrockPrxCfg)
			if err != nil {
				return err
			}
			proxies[infrared.BedrockEdition] = bedrockPrx

			pluginManager = infrared.PluginManager{
				Proxies: proxies,
				Plugins: []infrared.Plugin{
					&webhook.Plugin{},
					&prometheus.Plugin{},
					&api.Plugin{},
				},
				Logger: logger,
			}
			logger.Info("loading plugins")
			pluginManager.LoadPlugins(data)
			logger.Info("enabling plugins")
			pluginManager.EnablePlugins()
			defer pluginManager.DisablePlugins()

			logger.Info("starting proxies")
			for _, proxy := range proxies {
				go proxy.ListenAndServe(logger)
			}

			sc := make(chan os.Signal, 1)
			signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
			<-sc
			return nil
		},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&workingDir, "working-dir", "w", ".", "set the working directory")
	rootCmd.PersistentFlags().StringVarP(&environment, "environment", "e", "prod", "set the deployment environment")
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "config.yml", "path of the config file")

	rootCmd.AddCommand(licenseCmd)
	// Migration is not implemented yet
	// rootCmd.AddCommand(migrateCmd)
}

func initLogger() error {
	var err error
	switch environment {
	case "dev":
		logger, err = zap.NewDevelopment()
	case "prod":
		logger, err = zap.NewProduction()
	default:
		return fmt.Errorf("unsupported environment %q", environment)
	}
	return err
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

	javaPrxCfg, err := java.NewProxyConfigFromMap(cfg)
	if err != nil {
		logger.Error("failed to load java config",
			zap.Error(err),
		)
	}

	bedrockPrxCfg, err := bedrock.NewProxyConfigFromMap(cfg)
	if err != nil {
		logger.Error("failed to load bedrock config",
			zap.Error(err),
		)
	}

	prxCfgs := map[infrared.Edition]infrared.ProxyConfig{
		infrared.JavaEdition:    javaPrxCfg,
		infrared.BedrockEdition: bedrockPrxCfg,
	}

	logger.Info("Reloading proxies")
	for n, p := range proxies {
		if err := p.Reload(prxCfgs[n]); err != nil {
			logger.Error("failed to reload proxy",
				zap.Error(err),
			)
		}
	}

	logger.Info("Reloading plugins")
	pluginManager.ReloadPlugins(cfg)
}
