package cmd

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	synchers "github.com/uselagoon/lagoon-sync/synchers"
	"github.com/uselagoon/lagoon-sync/utils"
)

var ProjectName string
var sourceEnvironmentName string
var targetEnvironmentName string
var SyncerType string
var ServiceName string
var configurationFile string
var SSHHost string
var SSHPort string
var SSHKey string
var SSHVerbose bool
var noCliInteraction bool
var dryRun bool

var syncCmd = &cobra.Command{
	Use:   "sync [mariadb|files|mongodb|postgres|etc.]",
	Short: "Sync a resource type",
	Long:  `Use Lagoon-Sync to sync an external environments resources with the local environment`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		SyncerType := args[0]
		viper.Set("syncer-type", args[0])

		lagoonConfigBytestream, err := LoadLagoonConfig(cfgFile)
		if err != nil {
			utils.LogFatalError("Couldn't load lagoon config file - ", err.Error())
		}

		configRoot, err := synchers.UnmarshallLagoonYamlToLagoonSyncStructure(lagoonConfigBytestream)
		if err != nil {
			log.Fatalf("There was an issue unmarshalling the sync configuration from %v: %v", viper.ConfigFileUsed(), err)
		}

		// If no project flag is given, find project from env var.
		if ProjectName == "" {
			project, exists := os.LookupEnv("LAGOON_PROJECT")
			if exists {
				ProjectName = strings.Replace(project, "_", "-", -1)
			}
			if configRoot.Project != "" {
				ProjectName = configRoot.Project
			}
		}

		// Set service default to 'cli'
		if ServiceName == "" {
			ServiceName = getServiceName(SyncerType)
		}

		sourceEnvironment := synchers.Environment{
			ProjectName:     ProjectName,
			EnvironmentName: sourceEnvironmentName,
			ServiceName:     ServiceName,
		}

		// We assume that the target environment is local if it's not passed as an argument
		if targetEnvironmentName == "" {
			targetEnvironmentName = synchers.LOCAL_ENVIRONMENT_NAME
		}
		targetEnvironment := synchers.Environment{
			ProjectName:     ProjectName,
			EnvironmentName: targetEnvironmentName,
			ServiceName:     ServiceName,
		}

		var lagoonSyncer synchers.Syncer
		lagoonSyncer, err = synchers.GetSyncerForTypeFromConfigRoot(SyncerType, configRoot)
		if err != nil {
			utils.LogFatalError(err.Error(), nil)
		}

		if ProjectName == "" {
			utils.LogFatalError("No Project name given", nil)
		}

		if noCliInteraction == false {
			confirmationResult, err := confirmPrompt(fmt.Sprintf("Project: %s - you are about to sync %s from %s to %s, is this correct",
				ProjectName,
				SyncerType,
				sourceEnvironmentName, targetEnvironmentName))
			if err != nil || confirmationResult == false {
				utils.LogFatalError("User cancelled sync - exiting", nil)
			}
		}

		// SSH Config from file
		sshConfig := synchers.SSHConfig{}
		err = yaml.Unmarshal(lagoonConfigBytestream, &sshConfig)
		if err != nil {
			utils.LogFatalError(err.Error(), nil)
		}

		// SSH config
		var sshHost string
		if SSHHost != "" {
			sshHost = SSHHost
		} else if sshConfig.SSH.Host != "" {
			sshHost = sshConfig.SSH.Host
		} else {
			sshHost = "ssh.lagoon.amazeeio.cloud"
		}

		var sshPort string
		if SSHPort != "" {
			sshPort = SSHPort
		} else if sshConfig.SSH.Port != "" {
			sshPort = sshConfig.SSH.Port
		} else {
			sshPort = "3222"
		}

		var sshKey string
		if SSHKey != "" {
			sshKey = SSHKey
		} else if sshConfig.SSH.PrivateKey != "" {
			sshKey = sshConfig.SSH.PrivateKey
		}

		var sshVerbose bool
		if SSHVerbose {
			sshVerbose = SSHVerbose
		} else if sshConfig.SSH.Verbose {
			sshVerbose = sshConfig.SSH.Verbose
		}

		sshOptions := synchers.SSHOptions{
			Host:       sshHost,
			PrivateKey: sshKey,
			Port:       sshPort,
			Verbose:    sshVerbose,
		}

		err = synchers.RunSyncProcess(sourceEnvironment, targetEnvironment, lagoonSyncer, SyncerType, dryRun, sshOptions)
		if err != nil {
			utils.LogFatalError("There was an error running the sync process", err)
		}

		if !dryRun {
			log.Printf("\n------\nSuccessful sync of %s from %s to %s\n------", SyncerType, sourceEnvironment.GetOpenshiftProjectName(), targetEnvironment.GetOpenshiftProjectName())
		}
	},
}

func getServiceName(SyncerType string) string {
	if SyncerType == "mongodb" {
		return SyncerType
	}
	return "cli"
}

func confirmPrompt(message string) (bool, error) {
	prompt := promptui.Prompt{
		Label:     message,
		IsConfirm: true,
	}
	result, err := prompt.Run()
	if result == "y" {
		return true, err
	}
	return false, err
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.PersistentFlags().StringVarP(&ProjectName, "project-name", "p", "", "The Lagoon project name of the remote system")
	syncCmd.PersistentFlags().StringVarP(&sourceEnvironmentName, "source-environment-name", "e", "", "The Lagoon environment name of the source system")
	syncCmd.MarkPersistentFlagRequired("source-environment-name")
	syncCmd.PersistentFlags().StringVarP(&targetEnvironmentName, "target-environment-name", "t", "", "The target environment name (defaults to local)")
	syncCmd.PersistentFlags().StringVarP(&ServiceName, "service-name", "s", "", "The service name (default is 'cli'")
	syncCmd.PersistentFlags().StringVarP(&configurationFile, "configuration-file", "c", "", "File containing sync configuration.")
	syncCmd.MarkPersistentFlagRequired("remote-environment-name")
	syncCmd.PersistentFlags().StringVarP(&SSHHost, "ssh-host", "H", "", "Specify your lagoon ssh host, defaults to 'ssh.lagoon.amazeeio.cloud'")
	syncCmd.PersistentFlags().StringVarP(&SSHPort, "ssh-port", "P", "", "Specify your ssh port, defaults to '32222'")
	syncCmd.PersistentFlags().StringVarP(&SSHKey, "ssh-key", "i", "", "Specify path to a specific SSH key to use for authentication")
	syncCmd.PersistentFlags().BoolVar(&noCliInteraction, "no-interaction", false, "Disallow interaction")
	syncCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Don't run the commands, just preview what will be run")
	syncCmd.PersistentFlags().BoolVar(&SSHVerbose, "verbose", false, "Run ssh commands in verbose (useful for debugging)")

}
