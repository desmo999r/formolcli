/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"github.com/desmo999r/formolcli/controllers"
	"github.com/desmo999r/formolcli/session"
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
		session.CreateBackupSession(corev1.ObjectReference{
			Namespace: namespace,
			Name:      name,
		})
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

var backupSessionCmd = &cobra.Command{
	Use:   "backupsession",
	Short: "All the BackupSession related commands",
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
	backupSessionCmd.AddCommand(createBackupSessionCmd)
	rootCmd.AddCommand(startServerCmd)
	createBackupSessionCmd.Flags().String("namespace", "", "The namespace of the BackupConfiguration containing the information about the backup.")
	createBackupSessionCmd.Flags().String("name", "", "The name of the BackupConfiguration containing the information about the backup.")
	createBackupSessionCmd.MarkFlagRequired("namespace")
	createBackupSessionCmd.MarkFlagRequired("name")
}
