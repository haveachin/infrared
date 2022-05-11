package cmd

import (
	"embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/haveachin/infrared/internal/pkg/config"
	"github.com/haveachin/infrared/internal/plugin/webhook"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	v     *viper.Viper
	files embed.FS

	configPath string
	workingDir string

	rootCmd = &cobra.Command{
		Use:   "infrared",
		Short: "Starts the infrared proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, err := zap.NewDevelopment()
			if err != nil {
				return fmt.Errorf("failed to init logger; err: %s", err)
			}

			if err := os.Chdir(workingDir); err != nil {
				return err
			}

			logger.Info("loading proxy from config",
				zap.String("config", configPath),
			)

			if err := safeWriteFromEmbeddedFS("configs", "."); err != nil {
				return err
			}

			cfg, err := config.New(configPath)
			if err != nil {
				return err
			}

			prxCfgs, err := cfg.ReadProxyConfigs()
			if err != nil {
				return err
			}

			pluginManager := infrared.PluginManager{
				Plugins: []infrared.Plugin{
					&webhook.Plugin{
						Viper: v,
					},
				},
				Log: logger,
			}

			if err := pluginManager.EnablePlugins(); err != nil {
				return err
			}

			logger.Info("starting proxy")

			for _, prxCfg := range prxCfgs {
				prx, err := infrared.NewProxy(prxCfg)
				if err != nil {
					return err
				}
				go prx.ListenAndServe(logger)
			}

			sc := make(chan os.Signal, 1)
			signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
			<-sc

			logger.Info("disabling plugins")
			return pluginManager.DisablePlugins()
		},
	}
)

func init() {
	v = viper.New()
	v.SetEnvPrefix("INFRARED")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	rootCmd.Flags().StringVarP(&configPath, "config", "c", "config.yml", "path of the config file")
	rootCmd.Flags().StringVarP(&workingDir, "working-dir", "w", ".", "set the working directory")
	viper.BindPFlag("CONFIG", rootCmd.Flags().Lookup("config"))

	rootCmd.AddCommand(licenseCmd)
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
		if e.IsDir() {
			if _, err := os.Stat(sPath); !os.IsNotExist(err) {
				continue
			}

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
