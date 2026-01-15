package keeper

// Instance represents a running sandbox process
type Instance struct {
	Org         string
	Name        string // Repo Name
	IngressName string // From potfiles.ingress[].name
	Port        int
	Cmd         *JobCmd // Wrapper for creating process in a Job
}
