package iknite

type ClusterState int16

const (
	Undefined ClusterState = iota
	Stopped
	Started
	Initializing // Need to check the phase of the cluster
	Stabilizing
	Running
	Stopping
	Cleaning
	Failed
)

const (
	GroupName         = "iknite.kaweezle.com"
	V1alpha1Version   = "v1alpha1"
	IkniteClusterKind = "IkniteCluster"
)

func (s ClusterState) String() string {
	switch s {
	case Undefined:
		return "Undefined"
	case Stopped:
		return "Stopped"
	case Started:
		return "Started"
	case Initializing:
		return "Initializing"
	case Stabilizing:
		return "Stabilizing"
	case Running:
		return "Running"
	case Stopping:
		return "Stopping"
	case Cleaning:
		return "Cleaning"
	case Failed:
		return "Failed"
	}
	return "Unknown"
}

func (s *ClusterState) Set(value string) {
	switch value {
	case "Undefined":
		*s = Undefined
	case "Stopped":
		*s = Stopped
	case "Started":
		*s = Started
	case "Initializing":
		*s = Initializing
	case "Stabilizing":
		*s = Initializing
	case "Running":
		*s = Running
	case "Stopping":
		*s = Stopping
	case "Cleaning":
		*s = Cleaning
	case "Failed":
		*s = Failed
	}
}

func (s ClusterState) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

func (s *ClusterState) UnmarshalJSON(data []byte) error {
	value := string(data)
	value = value[1 : len(value)-1]
	s.Set(value)
	return nil
}
