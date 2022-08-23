package cmd

// CGFlags represents a set of flags custom to comparegroup
type CGFlags struct {
	// Name of Elastic cluster to find nodes in.
	ClusterName string

}

// NewCGFlags returns a new BaseFlags.
func NewCGFlags(app Flagger) *CGFlags {
	var flags CGFlags

	app.Flag("cg.cluster", "Name of Elastic cluster to use.").
		PlaceHolder("CG_CLUSTER").
		StringVar(&flags.ClusterName)

	return &flags
}

