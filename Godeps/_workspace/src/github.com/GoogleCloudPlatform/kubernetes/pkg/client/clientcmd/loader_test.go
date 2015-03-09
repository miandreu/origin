/*
Copyright 2014 Google Inc. All rights reserved.

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

package clientcmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"

	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	clientcmdlatest "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api/latest"
)

var (
	testConfigAlfa = clientcmdapi.Config{
		AuthInfos: map[string]clientcmdapi.AuthInfo{
			"red-user": {Token: "red-token"}},
		Clusters: map[string]clientcmdapi.Cluster{
			"cow-cluster": {Server: "http://cow.org:8080"}},
		Contexts: map[string]clientcmdapi.Context{
			"federal-context": {AuthInfo: "red-user", Cluster: "cow-cluster", Namespace: "hammer-ns"}},
	}
	testConfigBravo = clientcmdapi.Config{
		AuthInfos: map[string]clientcmdapi.AuthInfo{
			"black-user": {Token: "black-token"}},
		Clusters: map[string]clientcmdapi.Cluster{
			"pig-cluster": {Server: "http://pig.org:8080"}},
		Contexts: map[string]clientcmdapi.Context{
			"queen-anne-context": {AuthInfo: "black-user", Cluster: "pig-cluster", Namespace: "saw-ns"}},
	}
	testConfigCharlie = clientcmdapi.Config{
		AuthInfos: map[string]clientcmdapi.AuthInfo{
			"green-user": {Token: "green-token"}},
		Clusters: map[string]clientcmdapi.Cluster{
			"horse-cluster": {Server: "http://horse.org:8080"}},
		Contexts: map[string]clientcmdapi.Context{
			"shaker-context": {AuthInfo: "green-user", Cluster: "horse-cluster", Namespace: "chisel-ns"}},
	}
	testConfigDelta = clientcmdapi.Config{
		AuthInfos: map[string]clientcmdapi.AuthInfo{
			"blue-user": {Token: "blue-token"}},
		Clusters: map[string]clientcmdapi.Cluster{
			"chicken-cluster": {Server: "http://chicken.org:8080"}},
		Contexts: map[string]clientcmdapi.Context{
			"gothic-context": {AuthInfo: "blue-user", Cluster: "chicken-cluster", Namespace: "plane-ns"}},
	}

	testConfigConflictAlfa = clientcmdapi.Config{
		AuthInfos: map[string]clientcmdapi.AuthInfo{
			"red-user":    {Token: "a-different-red-token"},
			"yellow-user": {Token: "yellow-token"}},
		Clusters: map[string]clientcmdapi.Cluster{
			"cow-cluster":    {Server: "http://a-different-cow.org:8080", InsecureSkipTLSVerify: true},
			"donkey-cluster": {Server: "http://donkey.org:8080", InsecureSkipTLSVerify: true}},
		CurrentContext: "federal-context",
	}
)

func TestNonExistentCommandLineFile(t *testing.T) {
	loadingRules := ClientConfigLoadingRules{
		CommandLinePath: "bogus_file",
	}

	_, err := loadingRules.Load()
	if err == nil {
		t.Fatalf("Expected error for missing command-line file, got none")
	}
	if !strings.Contains(err.Error(), "bogus_file") {
		t.Fatalf("Expected error about 'bogus_file', got %s", err.Error())
	}
}

func TestToleratingMissingFiles(t *testing.T) {
	loadingRules := ClientConfigLoadingRules{
		EnvVarPath:           "bogus1",
		CurrentDirectoryPath: "bogus2",
		HomeDirectoryPath:    "bogus3",
	}

	_, err := loadingRules.Load()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestErrorReadingFile(t *testing.T) {
	commandLineFile, _ := ioutil.TempFile("", "")
	defer os.Remove(commandLineFile.Name())

	if err := ioutil.WriteFile(commandLineFile.Name(), []byte("bogus value"), 0644); err != nil {
		t.Fatalf("Error creating tempfile: %v", err)
	}

	loadingRules := ClientConfigLoadingRules{
		CommandLinePath: commandLineFile.Name(),
	}

	_, err := loadingRules.Load()
	if err == nil {
		t.Fatalf("Expected error for unloadable file, got none")
	}
	if !strings.Contains(err.Error(), commandLineFile.Name()) {
		t.Fatalf("Expected error about '%s', got %s", commandLineFile.Name(), err.Error())
	}
}

func TestErrorReadingNonFile(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Couldn't create tmpdir")
	}
	defer os.Remove(tmpdir)

	loadingRules := ClientConfigLoadingRules{
		CommandLinePath: tmpdir,
	}

	_, err = loadingRules.Load()
	if err == nil {
		t.Fatalf("Expected error for non-file, got none")
	}
	if !strings.Contains(err.Error(), tmpdir) {
		t.Fatalf("Expected error about '%s', got %s", tmpdir, err.Error())
	}
}

func TestConflictingCurrentContext(t *testing.T) {
	commandLineFile, _ := ioutil.TempFile("", "")
	defer os.Remove(commandLineFile.Name())
	envVarFile, _ := ioutil.TempFile("", "")
	defer os.Remove(envVarFile.Name())

	mockCommandLineConfig := clientcmdapi.Config{
		CurrentContext: "any-context-value",
	}
	mockEnvVarConfig := clientcmdapi.Config{
		CurrentContext: "a-different-context",
	}

	WriteToFile(mockCommandLineConfig, commandLineFile.Name())
	WriteToFile(mockEnvVarConfig, envVarFile.Name())

	loadingRules := ClientConfigLoadingRules{
		CommandLinePath: commandLineFile.Name(),
		EnvVarPath:      envVarFile.Name(),
	}

	mergedConfig, err := loadingRules.Load()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if mergedConfig.CurrentContext != mockCommandLineConfig.CurrentContext {
		t.Errorf("expected %v, got %v", mockCommandLineConfig.CurrentContext, mergedConfig.CurrentContext)
	}
}

func TestResolveRelativePaths(t *testing.T) {
	pathResolutionConfig1 := clientcmdapi.Config{
		AuthInfos: map[string]clientcmdapi.AuthInfo{
			"relative-user-1": {ClientCertificate: "relative/client/cert", ClientKey: "../relative/client/key", AuthPath: "../../relative/auth/path"},
			"absolute-user-1": {ClientCertificate: "/absolute/client/cert", ClientKey: "/absolute/client/key", AuthPath: "/absolute/auth/path"},
		},
		Clusters: map[string]clientcmdapi.Cluster{
			"relative-server-1": {CertificateAuthority: "../relative/ca"},
			"absolute-server-1": {CertificateAuthority: "/absolute/ca"},
		},
	}
	pathResolutionConfig2 := clientcmdapi.Config{
		AuthInfos: map[string]clientcmdapi.AuthInfo{
			"relative-user-2": {ClientCertificate: "relative/client/cert2", ClientKey: "../relative/client/key2", AuthPath: "../../relative/auth/path2"},
			"absolute-user-2": {ClientCertificate: "/absolute/client/cert2", ClientKey: "/absolute/client/key2", AuthPath: "/absolute/auth/path2"},
		},
		Clusters: map[string]clientcmdapi.Cluster{
			"relative-server-2": {CertificateAuthority: "../relative/ca2"},
			"absolute-server-2": {CertificateAuthority: "/absolute/ca2"},
		},
	}

	configDir1, _ := ioutil.TempDir("", "")
	configFile1 := path.Join(configDir1, ".kubeconfig")
	configDir1, _ = filepath.Abs(configDir1)
	defer os.Remove(configFile1)
	configDir2, _ := ioutil.TempDir("", "")
	configDir2, _ = ioutil.TempDir(configDir2, "")
	configFile2 := path.Join(configDir2, ".kubeconfig")
	configDir2, _ = filepath.Abs(configDir2)
	defer os.Remove(configFile2)

	WriteToFile(pathResolutionConfig1, configFile1)
	WriteToFile(pathResolutionConfig2, configFile2)

	loadingRules := ClientConfigLoadingRules{
		CommandLinePath: configFile1,
		EnvVarPath:      configFile2,
	}

	mergedConfig, err := loadingRules.Load()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	foundClusterCount := 0
	for key, cluster := range mergedConfig.Clusters {
		if key == "relative-server-1" {
			foundClusterCount++
			matchStringArg(path.Join(configDir1, pathResolutionConfig1.Clusters["relative-server-1"].CertificateAuthority), cluster.CertificateAuthority, t)
		}
		if key == "relative-server-2" {
			foundClusterCount++
			matchStringArg(path.Join(configDir2, pathResolutionConfig2.Clusters["relative-server-2"].CertificateAuthority), cluster.CertificateAuthority, t)
		}
		if key == "absolute-server-1" {
			foundClusterCount++
			matchStringArg(pathResolutionConfig1.Clusters["absolute-server-1"].CertificateAuthority, cluster.CertificateAuthority, t)
		}
		if key == "absolute-server-2" {
			foundClusterCount++
			matchStringArg(pathResolutionConfig2.Clusters["absolute-server-2"].CertificateAuthority, cluster.CertificateAuthority, t)
		}
	}
	if foundClusterCount != 4 {
		t.Errorf("Expected 4 clusters, found %v: %v", foundClusterCount, mergedConfig.Clusters)
	}

	foundAuthInfoCount := 0
	for key, authInfo := range mergedConfig.AuthInfos {
		if key == "relative-user-1" {
			foundAuthInfoCount++
			matchStringArg(path.Join(configDir1, pathResolutionConfig1.AuthInfos["relative-user-1"].ClientCertificate), authInfo.ClientCertificate, t)
			matchStringArg(path.Join(configDir1, pathResolutionConfig1.AuthInfos["relative-user-1"].ClientKey), authInfo.ClientKey, t)
			matchStringArg(path.Join(configDir1, pathResolutionConfig1.AuthInfos["relative-user-1"].AuthPath), authInfo.AuthPath, t)
		}
		if key == "relative-user-2" {
			foundAuthInfoCount++
			matchStringArg(path.Join(configDir2, pathResolutionConfig2.AuthInfos["relative-user-2"].ClientCertificate), authInfo.ClientCertificate, t)
			matchStringArg(path.Join(configDir2, pathResolutionConfig2.AuthInfos["relative-user-2"].ClientKey), authInfo.ClientKey, t)
			matchStringArg(path.Join(configDir2, pathResolutionConfig2.AuthInfos["relative-user-2"].AuthPath), authInfo.AuthPath, t)
		}
		if key == "absolute-user-1" {
			foundAuthInfoCount++
			matchStringArg(pathResolutionConfig1.AuthInfos["absolute-user-1"].ClientCertificate, authInfo.ClientCertificate, t)
			matchStringArg(pathResolutionConfig1.AuthInfos["absolute-user-1"].ClientKey, authInfo.ClientKey, t)
			matchStringArg(pathResolutionConfig1.AuthInfos["absolute-user-1"].AuthPath, authInfo.AuthPath, t)
		}
		if key == "absolute-user-2" {
			foundAuthInfoCount++
			matchStringArg(pathResolutionConfig2.AuthInfos["absolute-user-2"].ClientCertificate, authInfo.ClientCertificate, t)
			matchStringArg(pathResolutionConfig2.AuthInfos["absolute-user-2"].ClientKey, authInfo.ClientKey, t)
			matchStringArg(pathResolutionConfig2.AuthInfos["absolute-user-2"].AuthPath, authInfo.AuthPath, t)
		}
	}
	if foundAuthInfoCount != 4 {
		t.Errorf("Expected 4 users, found %v: %v", foundAuthInfoCount, mergedConfig.AuthInfos)
	}

}

func ExampleMergingSomeWithConflict() {
	commandLineFile, _ := ioutil.TempFile("", "")
	defer os.Remove(commandLineFile.Name())
	envVarFile, _ := ioutil.TempFile("", "")
	defer os.Remove(envVarFile.Name())

	WriteToFile(testConfigAlfa, commandLineFile.Name())
	WriteToFile(testConfigConflictAlfa, envVarFile.Name())

	loadingRules := ClientConfigLoadingRules{
		CommandLinePath: commandLineFile.Name(),
		EnvVarPath:      envVarFile.Name(),
	}

	mergedConfig, err := loadingRules.Load()

	json, err := clientcmdlatest.Codec.Encode(mergedConfig)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}
	output, err := yaml.JSONToYAML(json)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}

	fmt.Printf("%v", string(output))
	// Output:
	// apiVersion: v1
	// clusters:
	// - cluster:
	//     server: http://cow.org:8080
	//   name: cow-cluster
	// - cluster:
	//     insecure-skip-tls-verify: true
	//     server: http://donkey.org:8080
	//   name: donkey-cluster
	// contexts:
	// - context:
	//     cluster: cow-cluster
	//     namespace: hammer-ns
	//     user: red-user
	//   name: federal-context
	// current-context: federal-context
	// kind: Config
	// preferences: {}
	// users:
	// - name: red-user
	//   user:
	//     token: red-token
	// - name: yellow-user
	//   user:
	//     token: yellow-token
}

func ExampleMergingEverythingNoConflicts() {
	commandLineFile, _ := ioutil.TempFile("", "")
	defer os.Remove(commandLineFile.Name())
	envVarFile, _ := ioutil.TempFile("", "")
	defer os.Remove(envVarFile.Name())
	currentDirFile, _ := ioutil.TempFile("", "")
	defer os.Remove(currentDirFile.Name())
	homeDirFile, _ := ioutil.TempFile("", "")
	defer os.Remove(homeDirFile.Name())

	WriteToFile(testConfigAlfa, commandLineFile.Name())
	WriteToFile(testConfigBravo, envVarFile.Name())
	WriteToFile(testConfigCharlie, currentDirFile.Name())
	WriteToFile(testConfigDelta, homeDirFile.Name())

	loadingRules := ClientConfigLoadingRules{
		CommandLinePath:      commandLineFile.Name(),
		EnvVarPath:           envVarFile.Name(),
		CurrentDirectoryPath: currentDirFile.Name(),
		HomeDirectoryPath:    homeDirFile.Name(),
	}

	mergedConfig, err := loadingRules.Load()

	json, err := clientcmdlatest.Codec.Encode(mergedConfig)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}
	output, err := yaml.JSONToYAML(json)
	if err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}

	fmt.Printf("%v", string(output))
	// Output:
	// 	apiVersion: v1
	// clusters:
	// - cluster:
	//     server: http://chicken.org:8080
	//   name: chicken-cluster
	// - cluster:
	//     server: http://cow.org:8080
	//   name: cow-cluster
	// - cluster:
	//     server: http://horse.org:8080
	//   name: horse-cluster
	// - cluster:
	//     server: http://pig.org:8080
	//   name: pig-cluster
	// contexts:
	// - context:
	//     cluster: cow-cluster
	//     namespace: hammer-ns
	//     user: red-user
	//   name: federal-context
	// - context:
	//     cluster: chicken-cluster
	//     namespace: plane-ns
	//     user: blue-user
	//   name: gothic-context
	// - context:
	//     cluster: pig-cluster
	//     namespace: saw-ns
	//     user: black-user
	//   name: queen-anne-context
	// - context:
	//     cluster: horse-cluster
	//     namespace: chisel-ns
	//     user: green-user
	//   name: shaker-context
	// current-context: ""
	// kind: Config
	// preferences: {}
	// users:
	// - name: black-user
	//   user:
	//     token: black-token
	// - name: blue-user
	//   user:
	//     token: blue-token
	// - name: green-user
	//   user:
	//     token: green-token
	// - name: red-user
	//   user:
	//     token: red-token
}
