package synchers

import (
	"bytes"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"os/exec"
)

// UnmarshallLagoonYamlToLagoonSyncStructure will take a bytestream and return a fully parsed lagoon sync config structure
func UnmarshallLagoonYamlToLagoonSyncStructure(data []byte) (SyncherConfigRoot, error) {
	lagoonConfig := SyncherConfigRoot{
		LagoonSync: LagoonSync{},
	}
	err := yaml.Unmarshal(data, &lagoonConfig)
	if err != nil {
		return SyncherConfigRoot{}, errors.New("Unable to parse lagoon config yaml setup")
	}
	return lagoonConfig, nil
}

func RunSyncProcess(sourceEnvironment Environment, targetEnvironment Environment, lagoonSyncer Syncer) error {
	var err error
	err = SyncRunSourceCommand(sourceEnvironment, lagoonSyncer)

	if err != nil {
		_ = SyncCleanUp(lagoonSyncer)
		return err
	}
	err = SyncRunTransfer(sourceEnvironment, targetEnvironment, lagoonSyncer)
	if err != nil {
		_ = SyncCleanUp(lagoonSyncer)
		return err
	}

	err = SyncRunTargetCommand(targetEnvironment, lagoonSyncer)
	if err != nil {
		_ = SyncCleanUp(lagoonSyncer)
		return err
	}

	return SyncCleanUp(lagoonSyncer)
}

func SyncRunSourceCommand(remoteEnvironment Environment, syncer Syncer) error {
	var execString string

	if remoteEnvironment.EnvironmentName == LOCAL_ENVIRONMENT_NAME {
		execString = syncer.GetRemoteCommand(remoteEnvironment).command
	} else {
		execString = generateRemoteCommand(remoteEnvironment, syncer.GetRemoteCommand(remoteEnvironment).command)
	}

	err, outstring, errstring := Shellout(execString)

	if err != nil {
		fmt.Println(errstring)
		return err
	}
	fmt.Println(outstring)
	fmt.Println(execString)
	return nil
}

func SyncRunTransfer(sourceEnvironment Environment, targetEnvironment Environment, syncer Syncer) error {

	if sourceEnvironment.EnvironmentName == targetEnvironment.EnvironmentName {
		return nil
	}

	sourceEnvironmentName := syncer.GetTransferResource().Name
	if syncer.GetTransferResource().IsDirectory == true {
		sourceEnvironmentName += "/"
	}
	if sourceEnvironment.EnvironmentName != LOCAL_ENVIRONMENT_NAME {
		sourceEnvironmentName = fmt.Sprintf("%s@ssh.lagoon.amazeeio.cloud:%s", sourceEnvironment.getOpenshiftProjectName(), sourceEnvironmentName)
	}

	targetEnvironmentName := syncer.GetTransferResource().Name
	if targetEnvironment.EnvironmentName != LOCAL_ENVIRONMENT_NAME {
		targetEnvironmentName = fmt.Sprintf("%s@ssh.lagoon.amazeeio.cloud:%s", targetEnvironment.getOpenshiftProjectName(), targetEnvironmentName)
	}

	execString := fmt.Sprintf("rsync -e \"ssh -o LogLevel=ERROR -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -p 32222\" -a %s %s",
		sourceEnvironmentName,
		targetEnvironmentName)

	err, outstring, errstring := Shellout(execString)

	fmt.Println(outstring)

	if err != nil {
		fmt.Println(errstring)
		return err
	}

	fmt.Println(execString)
	return nil
}

func SyncRunTargetCommand(targetEnvironment Environment, syncer Syncer) error {
	//execString := syncer.GetLocalCommand(targetEnvironment)

	var execString string

	if targetEnvironment.EnvironmentName == LOCAL_ENVIRONMENT_NAME {
		execString = syncer.GetLocalCommand(targetEnvironment).command
	} else {
		execString = generateRemoteCommand(targetEnvironment, syncer.GetLocalCommand(targetEnvironment).command)
	}

	//err, outstring, errstring := Shellout(execString)
	//
	//if err != nil {
	//	fmt.Println(errstring)
	//	return err
	//}
	//fmt.Println(outstring)
	fmt.Println(execString)
	return nil
}

func SyncCleanUp(syncer Syncer) error {
	//remove remote resources
	//remove local resources
	fmt.Println("Cleaning up ...")
	return nil
}

func generateRemoteCommand(remoteEnvironment Environment, command string) string {
	return fmt.Sprintf("ssh -t -o \"UserKnownHostsFile=/dev/null\" -o \"StrictHostKeyChecking=no\" -p 32222 %v@ssh.lagoon.amazeeio.cloud '%v'",
		remoteEnvironment.getOpenshiftProjectName(), command)
}

const ShellToUse = "bash"

func Shellout(command string) (error, string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(ShellToUse, "-c", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return err, stdout.String(), stderr.String()
}
