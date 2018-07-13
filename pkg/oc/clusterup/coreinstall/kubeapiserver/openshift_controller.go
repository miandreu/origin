package kubeapiserver

import (
	"io/ioutil"
	"path"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/golang/glog"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/tmpformac"
)

func MakeOpenShiftControllerConfig(existingMasterConfig string, basedir string) (string, error) {
	configDir := path.Join(basedir, OpenShiftControllerManagerDirName)
	glog.V(1).Infof("Copying kube-apiserver config to local directory %s", OpenShiftControllerManagerDirName)
	if err := tmpformac.CopyDirectory(existingMasterConfig, configDir); err != nil {
		return "", err
	}

	// update some listen information to include starting the DNS server
	masterconfigFilename := path.Join(configDir, "master-config.yaml")
	originalBytes, err := ioutil.ReadFile(masterconfigFilename)
	if err != nil {
		return "", err
	}
	configObj, err := runtime.Decode(configapilatest.Codec, originalBytes)
	if err != nil {
		return "", err
	}
	masterconfig := configObj.(*configapi.MasterConfig)
	masterconfig.ServingInfo.BindAddress = "0.0.0.0:8444"

	// disable the service serving cert signer because that runs in a separate pod now
	masterconfig.ControllerConfig.Controllers = []string{
		"*",
		"-openshift.io/service-serving-cert",
	}

	configBytes, err := configapilatest.WriteYAML(masterconfig)
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(masterconfigFilename, configBytes, 0644); err != nil {
		return "", err
	}

	return configDir, nil
}
