package env

const (
	defaultAuthDirname     = "auth"
	defaultSharedDirname   = "shared"
	defaultImagesDirname   = "images"
	defaultClustersDirname = "clusters"
	defaultCACertFilename  = "ca.crt"
	defaultCAKeyFilename   = "ca.key"
	defaultValuesFilename  = "values.yaml"
	defaultKeyFilename     = "id_ed25519"
	defaultSecretsFilename = "secrets.sops.yaml" //nolint:gosec // This is just a filename.
)

type ClientConfigPaths struct {
	Root     string
	Auth     string
	Shared   string
	Images   string
	Clusters string

	CACert string
	CAKey  string

	SharedSecrets    string
	SharedSecretsKey string
	SharedValues     string
}
