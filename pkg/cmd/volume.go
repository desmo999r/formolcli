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
	"github.com/desmo999r/formolcli/pkg/restore"
	"github.com/spf13/cobra"
)

var volumeRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "restore a volume",
	Run: func(cmd *cobra.Command, args []string) {
		snapshotId, _ := cmd.Flags().GetString("snapshot-id")
		if err := restore.RestoreVolume(snapshotId); err != nil {
			os.Exit(1)
		}
	},
}

var volumeBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "backup a volume",
	Run: func(cmd *cobra.Command, args []string) {
		paths, _ := cmd.Flags().GetStringSlice("path")
		tag, _ := cmd.Flags().GetString("tag")
		if err := backup.BackupVolume(tag, paths); err != nil {
			os.Exit(1)
		}
	},
}

var volumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "volume actions",
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
	rootCmd.AddCommand(volumeCmd)
	volumeCmd.AddCommand(volumeBackupCmd)
	volumeCmd.AddCommand(volumeRestoreCmd)

	volumeBackupCmd.Flags().StringSlice("path", nil, "Path to the data to backup")
	volumeBackupCmd.Flags().String("tag", "", "Tag associated to the backup")
	volumeBackupCmd.MarkFlagRequired("path")
	volumeRestoreCmd.Flags().String("snapshot-id", "", "snapshot id associated to the backup")
	volumeRestoreCmd.MarkFlagRequired("snapshot-id")
}
