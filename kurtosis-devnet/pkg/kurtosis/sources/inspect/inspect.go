package inspect

import (
	"context"

	"github.com/kurtosis-tech/kurtosis/api/golang/engine/lib/kurtosis_context"
)

// PortInfo contains the host and port number for a service port
type PortInfo struct {
	Host string
	Port int
}

type PortMap map[string]PortInfo

type ServiceMap map[string]PortMap

// InspectData represents a summary of the output of "kurtosis enclave inspect"
type InspectData struct {
	FileArtifacts []string
	UserServices  ServiceMap
}

type Inspector struct {
	enclaveID string
}

func NewInspector(enclaveID string) *Inspector {
	return &Inspector{enclaveID: enclaveID}
}

func (e *Inspector) ExtractData(ctx context.Context) (*InspectData, error) {
	kurtosisCtx, err := kurtosis_context.NewKurtosisContextFromLocalEngine()
	if err != nil {
		return nil, err
	}

	enclaveCtx, err := kurtosisCtx.GetEnclaveContext(ctx, e.enclaveID)
	if err != nil {
		return nil, err
	}

	services, err := enclaveCtx.GetServices()
	if err != nil {
		return nil, err
	}

	artifacts, err := enclaveCtx.GetAllFilesArtifactNamesAndUuids(ctx)
	if err != nil {
		return nil, err
	}

	data := &InspectData{
		UserServices:  make(ServiceMap),
		FileArtifacts: make([]string, len(artifacts)),
	}

	for i, artifact := range artifacts {
		data.FileArtifacts[i] = artifact.GetFileName()
	}

	for svc := range services {
		svc := string(svc)
		svcCtx, err := enclaveCtx.GetServiceContext(svc)
		if err != nil {
			return nil, err
		}

		portMap := make(PortMap)

		for port, portSpec := range svcCtx.GetPublicPorts() {
			portMap[port] = PortInfo{
				Host: svcCtx.GetMaybePublicIPAddress(),
				Port: int(portSpec.GetNumber()),
			}
		}

		if len(portMap) != 0 {
			data.UserServices[svc] = portMap
		}

	}

	return data, nil
}
