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
	"fmt"

	"github.com/desmo999r/formolcli/pkg/backup"
	"github.com/spf13/cobra"
)

// postgresCmd represents the postgres command
var postgresCmd = &cobra.Command{
	Use:   "postgres",
	Short: "backup a PostgreSQL database",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("postgres called")
		file, _ := cmd.Flags().GetString("file")
		hostname, _ := cmd.Flags().GetString("hostname")
		database, _ := cmd.Flags().GetString("database")
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		_ = backup.BackupPostgres(file, hostname, database, username, password)
	},
}

func init() {
	backupCmd.AddCommand(postgresCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// postgresCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// postgresCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	postgresCmd.Flags().String("file", "", "The file the backup will be stored")
	postgresCmd.Flags().String("hostname", "", "The postgresql server host")
	postgresCmd.Flags().String("database", "", "The postgresql database")
	postgresCmd.Flags().String("username", "", "The postgresql username")
	postgresCmd.Flags().String("password", "", "The postgresql password")
	postgresCmd.MarkFlagRequired("path")
	postgresCmd.MarkFlagRequired("hostname")
	postgresCmd.MarkFlagRequired("database")
	postgresCmd.MarkFlagRequired("username")
	postgresCmd.MarkFlagRequired("password")
}
