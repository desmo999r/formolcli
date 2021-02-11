/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

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

	formolv1alpha1 "github.com/desmo999r/formol/api/v1alpha1"
	"github.com/desmo999r/formolcli/pkg/session"
	"github.com/spf13/cobra"
)

// targetCmd represents the target command
var targetFinalizeCmd = &cobra.Command{
	Use:   "finalize",
	Short: "Update the session target status",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("target called")
		session.RestoreSessionUpdateTargetStatus(formolv1alpha1.Success)
	},
}

var targetCmd = &cobra.Command{
	Use:   "target",
	Short: "A brief description of your command",
}

func init() {
	rootCmd.AddCommand(targetCmd)
	targetCmd.AddCommand(targetFinalizeCmd)
}
