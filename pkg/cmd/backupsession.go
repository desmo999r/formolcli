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
	"github.com/desmo999r/formolcli/pkg/server"
	"github.com/desmo999r/formolcli/pkg/session"
	"github.com/spf13/cobra"
)

var serverBackupSessionCmd = &cobra.Command{
	Use:   "server",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		server.Server()
	},
}

var createBackupSessionCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a backupsession",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		namespace, _ := cmd.Flags().GetString("namespace")
		session.CreateBackupSession(name, namespace)
	},
}

var backupSessionCmd = &cobra.Command{
	Use:   "backupsession",
	Short: "backupsession related commands",
}

func init() {
	rootCmd.AddCommand(backupSessionCmd)
	backupSessionCmd.AddCommand(createBackupSessionCmd)
	backupSessionCmd.AddCommand(serverBackupSessionCmd)
	createBackupSessionCmd.Flags().String("namespace", "", "The referenced BackupSessionConfiguration namespace")
	createBackupSessionCmd.Flags().String("name", "", "The referenced BackupSessionConfiguration name")
	createBackupSessionCmd.MarkFlagRequired("namespace")
	createBackupSessionCmd.MarkFlagRequired("name")
}
