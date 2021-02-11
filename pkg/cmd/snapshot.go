/* Copyright © 2020 NAME HERE <EMAIL ADDRESS>

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

	"github.com/desmo999r/formolcli/pkg/restic"
	"github.com/spf13/cobra"
)

// snapshotCmd represents the snapshot command
var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "A brief description of your command",
}

var snapshotDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete a snapshot",
	Run: func(cmd *cobra.Command, args []string) {
		snapshot, _ := cmd.Flags().GetString("snapshot-id")
		if err := restic.DeleteSnapshot(snapshot); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)
	snapshotDeleteCmd.Flags().String("snapshot-id", "", "The snapshot to delete")
	snapshotDeleteCmd.MarkFlagRequired("snapshot-id")
}
