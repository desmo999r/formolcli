/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"github.com/desmo999r/formolcli/controllers"
	"github.com/desmo999r/formolcli/standalone"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"os"
)

var createBackupSessionCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a backupsession",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		namespace, _ := cmd.Flags().GetString("namespace")
		fmt.Println("create backupsession called")
		standalone.CreateBackupSession(corev1.ObjectReference{
			Namespace: namespace,
			Name:      name,
		})
	},
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup paths",
	Run: func(cmd *cobra.Command, args []string) {
		backupSessionName, _ := cmd.Flags().GetString("name")
		backupSessionNamespace, _ := cmd.Flags().GetString("namespace")
		targetName, _ := cmd.Flags().GetString("target-name")
		standalone.BackupPaths(backupSessionName, backupSessionNamespace, targetName, args...)
	},
}

var startRestoreSessionCmd = &cobra.Command{
	Use:   "start",
	Short: "Restore a restic snapshot",
	Run: func(cmd *cobra.Command, args []string) {
		restoreSessionName, _ := cmd.Flags().GetString("name")
		restoreSessionNamespace, _ := cmd.Flags().GetString("namespace")
		targetName, _ := cmd.Flags().GetString("target-name")
		standalone.StartRestore(restoreSessionName, restoreSessionNamespace, targetName)
	},
}

var startServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a BackupSession / RestoreSession controller",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("starts backupsession controller")
		controllers.StartServer()
	},
}

var restoreSessionCmd = &cobra.Command{
	Use:   "restoresession",
	Short: "All the RestoreSession related commands",
}

var backupSessionCmd = &cobra.Command{
	Use:   "backupsession",
	Short: "All the BackupSession related commands",
}

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "All the snapshot related commands",
}

var deleteSnapshotCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a snapshot",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		namespace, _ := cmd.Flags().GetString("namespace")
		snapshotId, _ := cmd.Flags().GetString("snapshot-id")
		standalone.DeleteSnapshot(namespace, name, snapshotId)
	},
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "formolcli",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(backupSessionCmd)
	rootCmd.AddCommand(restoreSessionCmd)
	rootCmd.AddCommand(snapshotCmd)
	backupSessionCmd.AddCommand(createBackupSessionCmd)
	backupSessionCmd.AddCommand(backupCmd)
	restoreSessionCmd.AddCommand(startRestoreSessionCmd)
	snapshotCmd.AddCommand(deleteSnapshotCmd)
	rootCmd.AddCommand(startServerCmd)
	createBackupSessionCmd.Flags().String("namespace", "", "The namespace of the BackupConfiguration containing the information about the backup.")
	createBackupSessionCmd.Flags().String("name", "", "The name of the BackupConfiguration containing the information about the backup.")
	createBackupSessionCmd.MarkFlagRequired("namespace")
	createBackupSessionCmd.MarkFlagRequired("name")
	backupCmd.Flags().String("target-name", "", "The name of target being restored")
	backupCmd.Flags().String("namespace", "", "The namespace of the BackupConfiguration containing the information about the backup.")
	backupCmd.Flags().String("name", "", "The name of the BackupConfiguration containing the information about the backup.")
	backupCmd.MarkFlagRequired("namespace")
	backupCmd.MarkFlagRequired("name")
	backupCmd.MarkFlagRequired("target-name")
	startRestoreSessionCmd.Flags().String("namespace", "", "The namespace of RestoreSession")
	startRestoreSessionCmd.Flags().String("name", "", "The name of RestoreSession")
	startRestoreSessionCmd.Flags().String("target-name", "", "The name of target being restored")
	startRestoreSessionCmd.MarkFlagRequired("namespace")
	startRestoreSessionCmd.MarkFlagRequired("name")
	startRestoreSessionCmd.MarkFlagRequired("target-name")
	deleteSnapshotCmd.Flags().String("snapshot-id", "", "The snapshot id to delete")
	deleteSnapshotCmd.Flags().String("namespace", "", "The namespace of the BackupConfiguration containing the information about the backup.")
	deleteSnapshotCmd.Flags().String("name", "", "The name of the BackupConfiguration containing the information about the backup.")
	deleteSnapshotCmd.MarkFlagRequired("snapshot-id")
	deleteSnapshotCmd.MarkFlagRequired("namespace")
	deleteSnapshotCmd.MarkFlagRequired("name")
}
