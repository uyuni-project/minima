package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   string
	cfgFile   string
	cfgString string
)

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
