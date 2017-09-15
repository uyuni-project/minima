package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string
var cfgString string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "minima",
	Short: "A Simple Linux Repository Manager",
	Long:  "minima is an application to mirror and manage Linux package repos.",
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "minima.yaml", "config file")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// first, try from the commandline flag
	if cfgFile != "" {
		bytes, err := ioutil.ReadFile(cfgFile)
		if err != nil {
			log.Fatal(err)
		}
		cfgString = string(bytes)
		fmt.Println("Using config file:", cfgFile)
		return
	}

	// second, try via environment variable
	ev := os.Getenv("MINIMA_CONFIG")
	if ev != "" {
		cfgString = ev
		fmt.Println("Using configuration from $MINIMA_CONFIG")
		return
	}

	// third, try with minima.yaml
	bytes, err := ioutil.ReadFile("minima.yaml")
	if err != nil {
		log.Fatal(err)
	}
	cfgString = string(bytes)
	fmt.Println("Using config file:", cfgFile)
	return
}
