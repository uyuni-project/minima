package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/uyuni-project/minima/get"
	"github.com/uyuni-project/minima/updates"
	yaml "gopkg.in/yaml.v2"
)

var (
	version   string
	cfgFile   string
	cfgString string
)

const defaultTimeoutMinutes = 60

// Config maps the configuration in minima.yaml
type Config struct {
	Storage        get.StorageConfig
	SCC            get.SCC
	OBS            updates.OBS
	HTTP           []get.HTTPRepoConfig
	TimeoutMinutes uint `yaml:"timeout_minutes"`
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "minima",
	Short: "A Simple Linux Repository Manager",
	Long:  "minima is an application to mirror and manage Linux package repos.",
	Run: func(cmd *cobra.Command, args []string) {
		versionFlag, _ := cmd.Flags().GetBool("version")
		if versionFlag {
			fmt.Printf("minima %s\n", version)
			os.Exit(0)
		}

		cmd.Help()
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(versionTag string) {
	version = versionTag
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	// all sub-commands will have access to this flag
	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "minima.yaml", "config file")
	// local flags
	RootCmd.Flags().BoolP("version", "v", false, "Print minima version")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// first, try via environment variable
	ev := os.Getenv("MINIMA_CONFIG")
	if ev != "" {
		cfgString = ev
		fmt.Println("Using configuration from $MINIMA_CONFIG")
		return
	}

	// second, try from the commandline flag
	if cfgFile != "" {
		bytes, err := os.ReadFile(cfgFile)
		if err != nil {
			log.Fatal(err)
		}
		cfgString = string(bytes)
		fmt.Println("Using config file:", cfgFile)
	}
}

func parseConfig(configString string) (Config, error) {
	config := Config{}
	if err := yaml.Unmarshal([]byte(configString), &config); err != nil {
		return config, fmt.Errorf("configuration parse error: %v", err)
	}

	storageType := config.Storage.Type
	if storageType != "file" && storageType != "s3" {
		return config, fmt.Errorf("configuration parse error: unrecognised storage type")
	}

	if config.TimeoutMinutes == 0 {
		log.Printf("Applying default timeout of %d minutes to each request\n", defaultTimeoutMinutes)
		config.TimeoutMinutes = defaultTimeoutMinutes
	}
	return config, nil
}
