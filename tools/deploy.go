package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func GenerateRandomString(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate random string: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(b)[:length], nil
}

// ReadOrCreateFile reads the content of a file, or creates it with random content if it doesn't exist.
func ReadOrCreateFile(path string, length int) (string, error) {
	content, err := os.ReadFile(path)
	if err == nil {
		return string(content), nil
	}

	generatedString, err := GenerateRandomString(length)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(path, []byte(generatedString), 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write file %s: %v", path, err)
	}

	return generatedString, nil
}

// GetEnvOrFile reads an environment variable; if not present, reads from or creates a file.
func GetEnvOrFile(envName, filePath string, length int) (string, error) {
	val, exists := os.LookupEnv(envName)
	if exists && val != "" {
		return val, nil
	}

	return ReadOrCreateFile(filePath, length)
}

// GetEnvOrDefault returns the value of the environment variable named by the key
// or the default value if the environment variable is not set.
func GetEnvOrDefault(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if exists && value != "" {
		return value
	}

	return defaultValue
}

// GenerateHtpasswd generates a htpasswd string for the given username and password.
func GenerateHtpasswd(username, password string) (string, error) {
	cmd := exec.Command("htpasswd", "-n", "-b", "-B", username, password)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// execCommand is a utility function to execute external commands and capture their output.
func execCommand(command string, args ...string) {
	cmd := exec.Command(command, args...)
	var stdout, stderr bytes.Buffer
	// Capture standard output
	cmd.Stdout = &stdout
	// Capture standard error
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to execute command: %v, stdout: %s, stderr: %s", err, stdout.String(), stderr.String())
	}
}

// substituteEnvVars takes a template file path and a destination file path,
// reads the template, substitutes environment variable placeholders,
// and writes the result to the destination file.
func substituteEnvVars(templateFilePath, destFilePath string) error {
	templateContent, err := os.ReadFile(templateFilePath)
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	substitutedContent := os.ExpandEnv(string(templateContent))

	err = os.WriteFile(destFilePath, []byte(substitutedContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to write substituted content to file: %w", err)
	}

	return nil
}

// changeDirTemporarily changes the current working directory to the specified path and returns a function to revert to the original directory.
func changeDirTemporarily(newDir string) (revertFunc func(), err error) {
	originalDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	err = os.Chdir(newDir)
	if err != nil {
		return nil, err
	}

	return func() {
		err := os.Chdir(originalDir)
		if err != nil {
			log.Fatalf("Failed to revert to original directory: %v", err)
		}
	}, nil
}

func setupIronicDeployment(deployBasicAuthFlag, deployTLSFlag, deployKeepalivedFlag, deployMariadbFlag bool, tempIronicOverlay, kustomizePath string) {
	// Create a temporary overlay where we can make changes.
	revertFunc, err := changeDirTemporarily(tempIronicOverlay)
	if err != nil {
		log.Fatalf("Failed to change directory %s: %v", tempIronicOverlay, err)
	}
	defer revertFunc()

	execCommand(kustomizePath, "create", "--resources=../../../config/namespace", "--namespace=baremetal-operator-system", "--nameprefix=baremetal-operator-")

	if deployBasicAuthFlag {
		execCommand(kustomizePath, "edit", "add", "secret", "ironic-htpasswd", "--from-env-file=ironic-htpasswd")
		execCommand(kustomizePath, "edit", "add", "secret", "ironic-inspector-htpasswd", "--from-env-file=ironic-inspector-htpasswd")
		execCommand(kustomizePath, "edit", "add", "secret", "ironic-auth-config", "--from-file=auth-config=ironic-auth-config")
		execCommand(kustomizePath, "edit", "add", "secret", "ironic-inspector-auth-config", "--from-file=auth-config=ironic-inspector-auth-config")

		if deployTLSFlag {
			// Basic-auth + TLS is special since TLS also means reverse proxy, which affects basic-auth.
			// Therefore we have an overlay that we use as base for this case.
			execCommand(kustomizePath, "edit", "add", "resource", "../../overlays/basic-auth_tls")
		} else {
			execCommand(kustomizePath, "edit", "add", "resource", "../../base")
			execCommand(kustomizePath, "edit", "add", "component", "../../components/basic-auth")
		}
	} else if deployTLSFlag {
		execCommand(kustomizePath, "edit", "add", "component", "../../components/tls")
	}

	if deployKeepalivedFlag {
		execCommand(kustomizePath, "edit", "add", "component", "../../components/keepalived")
	}

	if deployMariadbFlag {
		execCommand(kustomizePath, "edit", "add", "component", "../../components/mariadb")
	}
}

func setupBMODeployment(deployBasicAuthFlag, deployTLSFlag bool, tempBMOOverlay, kustomizePath string) {
	// Create a temporary overlay where we can make changes.
	revertFunc, err := changeDirTemporarily(tempBMOOverlay)
	if err != nil {
		log.Fatalf("Failed to change directory %s: %v", tempBMOOverlay, err)
	}
	defer revertFunc()

	execCommand(kustomizePath, "create", "--resources=../../base,../../namespace", "--namespace=baremetal-operator-system")

	if deployBasicAuthFlag {
		execCommand(kustomizePath, "edit", "add", "component", "../../components/basic-auth")
		execCommand(kustomizePath, "edit", "add", "secret", "ironic-credentials", "--from-file=username=ironic-username", "--from-file=password=ironic-password")
		execCommand(kustomizePath, "edit", "add", "secret", "ironic-inspector-credentials", "--from-file=username=ironic-inspector-username", "--from-file=password=ironic-inspector-password")
	}

	if deployTLSFlag {
		execCommand(kustomizePath, "edit", "add", "component", "../../components/tls")
	}
}

// helper function to read lines from a file.
func readLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}

// helper function to write lines to a file.
func writeLines(lines []string, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}

	return w.Flush()
}

func updateOrAppendKeyValueInFile(filePath, key, value string) error {
	lines, err := readLines(filePath)
	if err != nil {
		return fmt.Errorf("failed to read lines from file %s: %v", filePath, err)
	}

	updatedLines := []string{}
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			updatedLines = append(updatedLines, fmt.Sprintf("%s=%s", key, value))
			found = true
		} else {
			updatedLines = append(updatedLines, line)
		}
	}
	if !found {
		updatedLines = append(updatedLines, fmt.Sprintf("%s=%s", key, value))
	}

	err = writeLines(updatedLines, filePath)
	if err != nil {
		return fmt.Errorf("failed to write updated lines to file %s: %v", filePath, err)
	}

	return nil
}

// validateCmd validates the command to ensure it doesn't contain harmful inputs.
func validateCmd(cmd []string) error {
	for _, arg := range cmd {
		if strings.Contains(arg, ";") || strings.Contains(arg, "&&") || strings.Contains(arg, "|") {
			return errors.New("invalid character in command arguments")
		}
	}

	return nil
}

// pipeCommands directly pipes the output of cmd1 to the input of cmd2.
func pipeCommands(cmd1 *exec.Cmd, baseCmd []string) error {
	err := validateCmd(baseCmd)
	if err != nil {
		return err
	}

	cmd2 := exec.Command(baseCmd[0], baseCmd[1:]...)

	pipeOut, err := cmd1.StdoutPipe()
	if err != nil {
		return err
	}
	defer pipeOut.Close()

	cmd2.Stdin = pipeOut

	cmd1.Stderr = os.Stderr
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr

	err = cmd1.Start()
	if err != nil {
		return err
	}

	err = cmd2.Start()
	if err != nil {
		return err
	}

	err = cmd1.Wait()
	if err != nil {
		return err
	}

	err = cmd2.Wait()
	if err != nil {
		return err
	}

	return nil
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %v", src, err)
	}

	err = os.WriteFile(dst, input, 0600)
	if err != nil {
		return fmt.Errorf("failed to write destination file %s: %v", dst, err)
	}

	return nil
}

func PrepareKubectlCmdArgs(baseCmd []string, kubectlArgs string) []string {
	if kubectlArgs != "" {
		baseCmd = append(baseCmd, strings.Split(kubectlArgs, " ")...)
	}

	return append(baseCmd, "-f", "-")
}

func deployBMO(tempBMOOverlay, scriptDir, kubectlArgs, kustomizePath string) error {
	revertFunc, err := changeDirTemporarily(tempBMOOverlay)
	if err != nil {
		log.Fatalf("Failed to change directory %s: %v", tempBMOOverlay, err)
	}
	defer revertFunc()

	// This is to keep the current behavior of using the ironic.env file for the configmap
	ironicEnvSrc := filepath.Join(scriptDir, "config", "default", "ironic.env")
	ironicEnvDst := filepath.Join(tempBMOOverlay, "ironic.env")
	copyFile(ironicEnvSrc, ironicEnvDst)

	execCommand(kustomizePath, "edit", "add", "configmap", "ironic", "--behavior=create", "--from-env-file=ironic.env")

	kubectlBaseCmd := []string{"kubectl", "apply"}
	kubectlBaseCmd = PrepareKubectlCmdArgs(kubectlBaseCmd, kubectlArgs)
	kustomizeBuildCmd := exec.Command(kustomizePath, "build", tempBMOOverlay)
	err = pipeCommands(kustomizeBuildCmd, kubectlBaseCmd)
	if err != nil {
		return err
	}

	return nil
}

func replaceAllPlaceholdersInFile(filePath, placeholder, newValue string) error {
	input, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %v", filePath, err)
	}

	output := strings.ReplaceAll(string(input), placeholder, newValue)
	err = os.WriteFile(filePath, []byte(output), 0600)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %v", filePath, err)
	}

	return nil
}

func deployIronic(tempIronicOverlay, scriptDir, kubectlArgs, kustomizePath string, deployKeepalivedFlag, deployTLSFlag bool, restartContainerCertificateUpdated, ironicHostIP, mariadbHostIP string) error {
	revertFunc, err := changeDirTemporarily(tempIronicOverlay)
	if err != nil {
		log.Fatalf("Failed to change directory %s: %v", tempIronicOverlay, err)
	}
	defer revertFunc()

	// Copy the configmap content from either the keepalived or default kustomization
	// and edit based on environment.
	var ironicBMOConfigmapSource string
	if deployKeepalivedFlag {
		ironicBMOConfigmapSource = filepath.Join(scriptDir, "ironic-deployment", "components", "keepalived", "ironic_bmo_configmap.env")
	} else {
		ironicBMOConfigmapSource = filepath.Join(scriptDir, "ironic-deployment", "default", "ironic_bmo_configmap.env")
	}
	ironicBMOConfigmap := filepath.Join(tempIronicOverlay, "ironic_bmo_configmap.env")
	copyFile(ironicBMOConfigmapSource, ironicBMOConfigmap)

	deployTLSStr := strconv.FormatBool(deployTLSFlag)
	err = updateOrAppendKeyValueInFile(ironicBMOConfigmap, "INSPECTOR_REVERSE_PROXY_SETUP", deployTLSStr)
	if err != nil {
		return err
	}

	err = updateOrAppendKeyValueInFile(ironicBMOConfigmap, "RESTART_CONTAINER_CERTIFICATE_UPDATED", restartContainerCertificateUpdated)
	if err != nil {
		return err
	}

	err = replaceAllPlaceholdersInFile(filepath.Join(scriptDir, "ironic-deployment", "components", "tls", "certificate.yaml"), "IRONIC_HOST_IP", ironicHostIP)
	if err != nil {
		return err
	}

	err = replaceAllPlaceholdersInFile(filepath.Join(scriptDir, "ironic-deployment", "components", "mariadb", "certificate.yaml"), "MARIADB_HOST_IP", mariadbHostIP)
	if err != nil {
		return err
	}

	// The keepalived component has its own configmap,
	// but we are overriding depending on environment here so we must replace it.
	if deployKeepalivedFlag {
		execCommand(kustomizePath, "edit", "add", "configmap", "ironic-bmo-configmap", "--behavior=replace", "--from-env-file=ironic_bmo_configmap.env")
	} else {
		execCommand(kustomizePath, "edit", "add", "configmap", "ironic-bmo-configmap", "--behavior=create", "--from-env-file=ironic_bmo_configmap.env")
	}

	kubectlBaseCmd := []string{"kubectl", "apply"}
	kubectlBaseCmd = PrepareKubectlCmdArgs(kubectlBaseCmd, kubectlArgs)
	kustomizeBuildCmd := exec.Command(kustomizePath, "build", tempIronicOverlay)
	err = pipeCommands(kustomizeBuildCmd, kubectlBaseCmd)
	if err != nil {
		return err
	}

	return nil
}

func cleanup(deployBasicAuthFlag, deployBMOFlag, deployIronicFlag bool, tempBMOOverlay, tempIronicOverlay string) {
	if deployBasicAuthFlag {
		if deployBMOFlag {
			os.Remove(filepath.Join(tempBMOOverlay, "ironic-username"))
			os.Remove(filepath.Join(tempBMOOverlay, "ironic-password"))
			os.Remove(filepath.Join(tempBMOOverlay, "ironic-inspector-username"))
			os.Remove(filepath.Join(tempBMOOverlay, "ironic-inspector-password"))
		}

		if deployIronicFlag {
			os.Remove(filepath.Join(tempIronicOverlay, "ironic-auth-config"))
			os.Remove(filepath.Join(tempIronicOverlay, "ironic-inspector-auth-config"))
			os.Remove(filepath.Join(tempIronicOverlay, "ironic-htpasswd"))
			os.Remove(filepath.Join(tempIronicOverlay, "ironic-inspector-htpasswd"))
		}
	}
}

func usage() {
	fmt.Println(`Usage : deploy [options]
Options:
	-h:	Show this help message
	-b:	deploy BMO
	-i:	deploy Ironic
	-t:	deploy with TLS enabled
	-n:	deploy without authentication
	-k:	deploy with keepalived
	-m:	deploy with mariadb (requires TLS enabled)`)
}

var (
	deployBMOFlag         bool
	deployIronicFlag      bool
	deployTLSFlag         bool
	deployWithoutAuthFlag bool
	deployKeepalivedFlag  bool
	deployMariadbFlag     bool
	showHelpFlag          bool
)

func main() {
	flag.BoolVar(&deployBMOFlag, "b", false, "Deploy BMO")
	flag.BoolVar(&deployIronicFlag, "i", false, "Deploy Ironic")
	flag.BoolVar(&deployTLSFlag, "t", false, "Deploy with TLS enabled")
	flag.BoolVar(&deployWithoutAuthFlag, "n", false, "Deploy with authentication disabled")
	flag.BoolVar(&deployKeepalivedFlag, "k", false, "Deploy with keepalived")
	flag.BoolVar(&deployMariadbFlag, "m", false, "Deploy with mariadb (requires TLS enabled)")
	flag.BoolVar(&showHelpFlag, "h", false, "Show help message")

	flag.Usage = usage
	flag.Parse()

	deployBasicAuthFlag := !deployWithoutAuthFlag

	if showHelpFlag {
		usage()
		return
	}

	if !deployBasicAuthFlag {
		fmt.Println("WARNING: Deploying without authentication is not recommended")
	}

	if deployMariadbFlag && !deployTLSFlag {
		fmt.Println("ERROR: Deploying Ironic with MariaDB without TLS is not supported.")
		usage()
		os.Exit(1)
	}

	if !deployBMOFlag && !deployIronicFlag {
		fmt.Println("ERROR: At least one of -b (BMO) or -i (Ironic) must be specified for deployment.")
		usage()
		os.Exit(1)
	}

	// Environment variables
	mariaDBHostIP := GetEnvOrDefault("MARIADB_HOST_IP", "127.0.0.1")
	kubectlArgs := GetEnvOrDefault("KUBECTL_ARGS", "")
	restartContainerCertificateUpdated := GetEnvOrDefault("RESTART_CONTAINER_CERTIFICATE_UPDATED", "false")

	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to determine executable path: %v", err)
	}
	scriptDir := filepath.Join(filepath.Dir(exePath), "..")

	kustomizePath := filepath.Join(scriptDir, "tools", "bin", "kustomize")

	ironicBasicAuthComponent := filepath.Join(scriptDir, "ironic-deployment", "components", "basic-auth")
	tempBMOOverlay := filepath.Join(scriptDir, "config", "overlays", "temp")
	tempIronicOverlay := filepath.Join(scriptDir, "ironic-deployment", "overlays", "temp")

	// Cleaning up directories
	err = os.RemoveAll(tempBMOOverlay)
	if err != nil {
		log.Fatalf("Failed to remove temp BMO overlay: %v", err)
	}
	err = os.RemoveAll(tempIronicOverlay)
	if err != nil {
		log.Fatalf("Failed to remove temp Ironic overlay: %v", err)
	}

	err = os.MkdirAll(tempBMOOverlay, 0755)
	if err != nil {
		log.Fatalf("Failed to create temp BMO overlay directory: %v", err)
	}
	err = os.MkdirAll(tempIronicOverlay, 0755)
	if err != nil {
		log.Fatalf("Failed to create temp Ironic overlay directory: %v", err)
	}

	execCommand("make", "-C", scriptDir, kustomizePath)

	ironicDataDir := os.Getenv("IRONIC_DATA_DIR")
	if ironicDataDir == "" {
		ironicDataDir = "/opt/metal3/ironic/"
	}
	ironicAuthDir := filepath.Join(ironicDataDir, "auth")

	if deployBasicAuthFlag {
		ironicUsername, err := GetEnvOrFile("IRONIC_USERNAME", filepath.Join(ironicAuthDir, "ironic-username"), 12)
		if err != nil {
			log.Fatalf("Error retrieving Ironic username: %v", err)
		}
		ironicPassword, err := GetEnvOrFile("IRONIC_PASSWORD", filepath.Join(ironicAuthDir, "ironic-password"), 12)
		if err != nil {
			log.Fatalf("Error retrieving Ironic password: %v", err)
		}
		ironicInspectorUsername, err := GetEnvOrFile("IRONIC_INSPECTOR_USERNAME", filepath.Join(ironicAuthDir, "ironic-inspector-username"), 12)
		if err != nil {
			log.Fatalf("Error retrieving Ironic Inspector username: %v", err)
		}
		ironicInspectorPassword, err := GetEnvOrFile("IRONIC_INSPECTOR_PASSWORD", filepath.Join(ironicAuthDir, "ironic-inspector-password"), 12)
		if err != nil {
			log.Fatalf("Error retrieving Ironic Inspector password: %v", err)
		}

		if deployBMOFlag {
			tempBMOOverlayPath := filepath.Join(tempBMOOverlay, "ironic-username")
			err = os.WriteFile(tempBMOOverlayPath, []byte(ironicUsername), 0600)
			if err != nil {
				log.Fatalf("Failed to write BMO overlay file: %v", err)
			}
			tempBMOOverlayPath = filepath.Join(tempBMOOverlay, "ironic-password")
			err = os.WriteFile(tempBMOOverlayPath, []byte(ironicPassword), 0600)
			if err != nil {
				log.Fatalf("Failed to write BMO overlay file: %v", err)
			}
			tempBMOOverlayPath = filepath.Join(tempBMOOverlay, "ironic-inspector-username")
			err = os.WriteFile(tempBMOOverlayPath, []byte(ironicInspectorUsername), 0600)
			if err != nil {
				log.Fatalf("Failed to write BMO overlay file: %v", err)
			}
			tempBMOOverlayPath = filepath.Join(tempBMOOverlay, "ironic-inspector-password")
			err = os.WriteFile(tempBMOOverlayPath, []byte(ironicInspectorPassword), 0600)
			if err != nil {
				log.Fatalf("Failed to write BMO overlay file: %v", err)
			}
		}

		if deployIronicFlag {
			err := substituteEnvVars(
				filepath.Join(ironicBasicAuthComponent, "ironic-auth-config-tpl"),
				filepath.Join(tempIronicOverlay, "ironic-auth-config"),
			)
			if err != nil {
				log.Fatalf("Failed to process ironic-auth-config template: %v", err)
			}

			err = substituteEnvVars(
				filepath.Join(ironicBasicAuthComponent, "ironic-inspector-auth-config-tpl"),
				filepath.Join(tempIronicOverlay, "ironic-inspector-auth-config"),
			)
			if err != nil {
				log.Fatalf("Failed to process ironic-inspector-auth-config template: %v", err)
			}

			ironicHtpasswd, err := GenerateHtpasswd(ironicUsername, ironicPassword)
			if err != nil {
				log.Fatalf("Failed to generate ironic htpasswd: %v", err)
			}
			inspectorHtpasswd, err := GenerateHtpasswd(ironicInspectorUsername, ironicInspectorPassword)
			if err != nil {
				log.Fatalf("Failed to generate inspector htpasswd: %v", err)
			}

			htpasswdPath := filepath.Join(tempIronicOverlay, "ironic-htpasswd")
			err = os.WriteFile(htpasswdPath, []byte("IRONIC_HTPASSWD="+ironicHtpasswd), 0600)
			if err != nil {
				log.Fatalf("Failed to write ironic htpasswd file: %v", err)
			}

			inspectorHtpasswdPath := filepath.Join(tempIronicOverlay, "ironic-inspector-htpasswd")
			err = os.WriteFile(inspectorHtpasswdPath, []byte("INSPECTOR_HTPASSWD="+inspectorHtpasswd), 0600)
			if err != nil {
				log.Fatalf("Failed to write inspector htpasswd file: %v", err)
			}
		}
	}

	if deployBMOFlag {
		setupBMODeployment(deployBasicAuthFlag, deployTLSFlag, tempBMOOverlay, kustomizePath)

		err = deployBMO(tempBMOOverlay, scriptDir, kubectlArgs, kustomizePath)
		if err != nil {
			log.Fatalf("Failed to deploy BMO: %v", err)
		}
	}

	if deployIronicFlag {
		setupIronicDeployment(deployBasicAuthFlag, deployTLSFlag, deployKeepalivedFlag, deployMariadbFlag, tempIronicOverlay, kustomizePath)

		err = deployIronic(tempIronicOverlay, scriptDir, kubectlArgs, kustomizePath, deployKeepalivedFlag, deployTLSFlag, restartContainerCertificateUpdated, mariaDBHostIP, "IRONIC_HOST_IP_VALUE")
		if err != nil {
			log.Fatalf("Failed to deploy Ironic: %v", err)
		}
	}

	cleanup(deployBasicAuthFlag, deployBMOFlag, deployIronicFlag, tempBMOOverlay, tempIronicOverlay)
}
