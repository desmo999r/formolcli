/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"os"

	"github.com/desmo999r/formolcli/pkg/backup"
	"github.com/spf13/cobra"
)

// pvcCmd represents the pvc command
var volumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		paths, _ := cmd.Flags().GetStringSlice("path")
		tag, _ := cmd.Flags().GetString("tag")
		if err := backup.BackupVolume(tag, paths); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	backupCmd.AddCommand(volumeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// pvcCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// pvcCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	volumeCmd.Flags().StringSlice("path", nil, "Path to the data to backup")
	volumeCmd.Flags().String("tag", "", "Tag associated to the backup")
	volumeCmd.MarkFlagRequired("path")
}