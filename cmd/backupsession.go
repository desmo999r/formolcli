/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"github.com/desmo999r/formolcli/backupsession"
	"github.com/desmo999r/formolcli/controllers"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
)

var createBackupSessionCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a backupsession",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		namespace, _ := cmd.Flags().GetString("namespace")
		fmt.Println("create backupsession called")
		backupsession.CreateBackupSession(corev1.ObjectReference{
			Namespace: namespace,
			Name:      name,
		})
	},
}

var startServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a BackupSession controller",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("starts backupsession controller")
		controllers.StartServer()
	},
}

// backupsessionCmd represents the backupsession command
var backupSessionCmd = &cobra.Command{
	Use:   "backupsession",
	Short: "All the BackupSession related commands",
}

func init() {
	rootCmd.AddCommand(backupSessionCmd)
	backupSessionCmd.AddCommand(createBackupSessionCmd)
	backupSessionCmd.AddCommand(startServerCmd)
	createBackupSessionCmd.Flags().String("namespace", "", "The namespace of the BackupConfiguration containing the information about the backup.")
	createBackupSessionCmd.Flags().String("name", "", "The name of the BackupConfiguration containing the information about the backup.")
	createBackupSessionCmd.MarkFlagRequired("namespace")
	createBackupSessionCmd.MarkFlagRequired("name")
}
