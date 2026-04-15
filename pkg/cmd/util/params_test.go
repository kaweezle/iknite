package util_test

// cSpell: words pflag myflag mysection testcmd testapp paralleltest
import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/cmd/util"
)

func newStringFlag(t *testing.T, name string) *pflag.Flag {
	t.Helper()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String(name, "", "test flag")
	flag := flags.Lookup(name)
	require.NotNil(t, flag)
	return flag
}

func TestGetBaseDirectory(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	// The test must run from a directory inside a git repo (the karmafun repo itself).
	dir, err := util.GetBaseDirectory()
	req.NoError(err)
	req.NotEmpty(dir)
	// The returned directory should contain a .git directory.
	_, err = os.Stat(dir + "/.git")
	req.NoErrorf(err, "base directory %q should contain a .git directory", dir)
}

//nolint:paralleltest // This test changes the current working directory,.
func TestGetBaseDirectory_OutsideGitRepo(t *testing.T) {
	req := require.New(t)
	// Change to a temp dir that is not inside a git repo.
	tmpDir, err := os.MkdirTemp("", "no-git-")
	req.NoError(err)
	defer func() {
		req.NoError(os.RemoveAll(tmpDir))
	}()

	originalDir, err := os.Getwd()
	req.NoError(err)
	defer func() {
		req.NoError(os.Chdir(originalDir))
	}()

	req.NoError(os.Chdir(tmpDir))
	dir, err := util.GetBaseDirectory()
	req.NoError(err)
	req.Equal(".", dir, "should return '.' when no git dir found")
}

// --- CommandConfigSection tests ---

func TestSetCommandConfigSection_SetsAnnotation(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cmd := &cobra.Command{Use: "test"}
	util.SetCommandConfigSection(cmd, "my-section")
	req.Equal("my-section", cmd.Annotations[util.ConfigSectionAnnotation])
}

func TestCommandHasConfigSection_True(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cmd := &cobra.Command{Use: "test"}
	util.SetCommandConfigSection(cmd, "my-section")
	req.True(util.CommandHasConfigSection(cmd))
}

func TestCommandHasConfigSection_False_NoAnnotations(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cmd := &cobra.Command{Use: "test"}
	req.False(util.CommandHasConfigSection(cmd))
}

func TestCommandConfigSection_ReturnsSection(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cmd := &cobra.Command{Use: "test"}
	util.SetCommandConfigSection(cmd, "my-section")
	req.Equal("my-section", util.CommandConfigSection(cmd))
}

func TestCommandConfigSection_ReturnsEmpty_WhenNoAnnotation(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cmd := &cobra.Command{Use: "test"}
	req.Empty(util.CommandConfigSection(cmd))
}

func TestCommandConfigSection_ReturnsEmpty_WhenAnnotationsDoNotContainSection(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cmd := &cobra.Command{Use: "test", Annotations: map[string]string{"other": "value"}}
	req.Empty(util.CommandConfigSection(cmd))
}

// --- SetSkipViperBind tests ---

func TestSetSkipViperBindForCommand_Skip(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cmd := &cobra.Command{Use: "test"}
	util.SetSkipViperBindForCommand(cmd, true)
	req.True(util.CmdShouldSkipViperBind(cmd))
}

func TestSetSkipViperBindForCommand_NoSkip(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cmd := &cobra.Command{Use: "test"}
	util.SetSkipViperBindForCommand(cmd, true)
	util.SetSkipViperBindForCommand(cmd, false)
	req.False(util.CmdShouldSkipViperBind(cmd))
}

func TestCmdShouldSkipViperBind_False_NoAnnotations(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cmd := &cobra.Command{Use: "test"}
	req.False(util.CmdShouldSkipViperBind(cmd))
}

func TestCmdShouldSkipViperBind_False_WhenAnnotationMissing(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cmd := &cobra.Command{Use: "test", Annotations: map[string]string{"other": "value"}}
	req.False(util.CmdShouldSkipViperBind(cmd))
}

func TestSetSkipViperBindForFlag_Skip(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("myflag", "", "test flag")
	flag := flags.Lookup("myflag")
	req.NotNil(flag)
	util.SetSkipViperBindForFlag(flag, true)
	req.True(util.FlagShouldSkipViperBind(flag))
}

func TestSetSkipViperBindForFlag_NoSkip(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("myflag", "", "test flag")
	flag := flags.Lookup("myflag")
	req.NotNil(flag)
	util.SetSkipViperBindForFlag(flag, true)
	util.SetSkipViperBindForFlag(flag, false)
	req.False(util.FlagShouldSkipViperBind(flag))
}

func TestFlagShouldSkipViperBind_False_NoAnnotation(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("myflag", "", "test flag")
	flag := flags.Lookup("myflag")
	req.NotNil(flag)
	req.False(util.FlagShouldSkipViperBind(flag))
}

func TestFlagShouldSkipViperBind_False_WhenAnnotationMissing(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	flag := newStringFlag(t, "myflag")
	flag.Annotations = map[string][]string{"other": {"value"}}
	req.False(util.FlagShouldSkipViperBind(flag))
}

// --- BindFlagValue tests ---

func TestBindFlagValue_AppliesViperValueToFlag(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("my_flag", "from-viper")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("my-flag", "default", "test flag")
	flag := flags.Lookup("my-flag")
	req.NotNil(flag)

	err := util.BindFlagValue(flag, v, "my_flag")
	req.NoError(err)
	req.Equal("from-viper", flag.Value.String())
}

func TestBindFlagValue_SkipsWhenFlagChanged(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("my_flag", "from-viper")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("my-flag", "default", "test flag")
	err := flags.Set("my-flag", "from-cli")
	req.NoError(err)
	flag := flags.Lookup("my-flag")
	req.NotNil(flag)
	req.True(flag.Changed)

	err = util.BindFlagValue(flag, v, "my_flag")
	req.NoError(err)
	req.Equal("from-cli", flag.Value.String(), "flag changed by CLI should not be overridden by viper")
}

func TestBindFlagValue_SkipsWhenViperKeyNotSet(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("my-flag", "default", "test flag")
	flag := flags.Lookup("my-flag")
	req.NotNil(flag)

	err := util.BindFlagValue(flag, v, "my_flag")
	req.NoError(err)
	req.Equal("default", flag.Value.String(), "should keep default when viper key is not set")
}

func TestBindFlagValue_SliceFlag(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("my_slice", []string{"a", "b", "c"})

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.StringSlice("my-slice", []string{}, "test slice flag")
	flag := flags.Lookup("my-slice")
	req.NotNil(flag)

	err := util.BindFlagValue(flag, v, "my_slice")
	req.NoError(err)
	req.Equal("[a,b,c]", flag.Value.String())
}

func TestBindFlagValue_SliceFlag_FromEnvironmentStyleString(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("my_slice", "a,b,c")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.StringSlice("my-slice", []string{}, "test slice flag")
	flag := flags.Lookup("my-slice")
	req.NotNil(flag)

	err := util.BindFlagValue(flag, v, "my_slice")
	req.NoError(err)
	req.Equal("[a,b,c]", flag.Value.String())
}

func TestBindFlagValue_SliceFlag_FromGenericSlice(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("my_slice", []any{"a", 2, true})

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.StringSlice("my-slice", []string{}, "test slice flag")
	flag := flags.Lookup("my-slice")
	req.NotNil(flag)

	err := util.BindFlagValue(flag, v, "my_slice")
	req.NoError(err)
	req.Equal("[a,2,true]", flag.Value.String())
}

// --- AddConfigFlag tests ---

func TestAddConfigFlag_AddsFlag(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	cmd := &cobra.Command{Use: "testcmd"}
	util.AddConfigFlag(cmd)
	flag := cmd.PersistentFlags().Lookup(util.ConfigFlag)
	req.NotNil(flag, "config flag should be added to command")
	req.Equal("c", flag.Shorthand)
}

// --- BindFlagsToViper / ApplyViperConfigToFlags ---

func TestBindFlags_SkipsCommandWithSkipAnnotation(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("my_flag", "from-viper")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("my-flag", "default", "test flag")
	util.SetSkipViperBindForCommand(cmd, true)

	util.ApplyViperConfigToFlags(cmd, v)

	flag := cmd.Flags().Lookup("my-flag")
	req.NotNil(flag)
	req.Equal("default", flag.Value.String(), "flag in skipped command should not be changed")
}

func TestBindFlags_AppliesViperValuesToFlags(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("my_flag", "from-viper")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("my-flag", "default", "test flag")

	util.ApplyViperConfigToFlags(cmd, v)

	flag := cmd.Flags().Lookup("my-flag")
	req.NotNil(flag)
	req.Equal("from-viper", flag.Value.String())
}

func TestBindFlags_SkipsFlagWithSkipAnnotation(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("my_flag", "from-viper")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("my-flag", "default", "test flag")
	flag := cmd.Flags().Lookup("my-flag")
	req.NotNil(flag)
	util.SetSkipViperBindForFlag(flag, true)

	util.ApplyViperConfigToFlags(cmd, v)

	req.Equal("default", flag.Value.String(), "flag with skip annotation should not be changed")
}

func TestBindFlags_UsesConfigSectionAsPrefix(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("mysection.my_flag", "from-section")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("my-flag", "default", "test flag")
	util.SetCommandConfigSection(cmd, "mysection")

	util.ApplyViperConfigToFlags(cmd, v)

	flag := cmd.Flags().Lookup("my-flag")
	req.NotNil(flag)
	req.Equal("from-section", flag.Value.String())
}

func TestBindFlags_Recurses_Subcommands(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()
	v.Set("child_flag", "child-from-viper")

	parent := &cobra.Command{Use: "parent"}
	child := &cobra.Command{Use: "child"}
	child.Flags().String("child-flag", "default", "child test flag")
	parent.AddCommand(child)

	util.ApplyViperConfigToFlags(parent, v)

	flag := child.Flags().Lookup("child-flag")
	req.NotNil(flag)
	req.Equal("child-from-viper", flag.Value.String())
}

func TestBindFlags_ContinuesWhenBinderReturnsError(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()

	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("persistent-flag", "default", "persistent test flag")
	cmd.Flags().String("my-flag", "default", "test flag")

	called := map[string]string{}
	binder := func(f *pflag.Flag, _ *viper.Viper, viperName string) error {
		called[f.Name] = viperName
		return fmt.Errorf("bind %s: %w", f.Name, errors.New("boom"))
	}

	util.BindFlags(cmd, v, "prefix.", binder)

	req.Equal(map[string]string{
		"persistent-flag": "prefix.persistent_flag",
		"my-flag":         "prefix.my_flag",
	}, called)
	flag := cmd.Flags().Lookup("my-flag")
	req.NotNil(flag)
	req.Equal("default", flag.Value.String())
}

// --- BindFlag tests ---

func TestBindFlag_BindsPersistentFlag(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("my-flag", "default", "test flag")
	flag := flags.Lookup("my-flag")
	req.NotNil(flag)

	err := util.BindFlag(flag, v, "my_flag")
	req.NoError(err)

	v.Set("my_flag", "from-viper")
	req.Equal("from-viper", v.GetString("my_flag"))
}

// --- BindFlagsToViper tests ---

func TestBindFlagsToViper_BindsFlags(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()

	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("my-flag", "default", "test flag")

	util.BindFlagsToViper(cmd, v)
	// After binding, setting viper value should be accessible
	v.Set("my_flag", "bound-value")
	req.Equal("bound-value", v.GetString("my_flag"))
}

func TestBindFlagsToViper_BindsSubcommandFlags(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()

	rootCmd := &cobra.Command{Use: "root"}
	childCmd := &cobra.Command{Use: "child"}
	childCmd.Flags().String("child-flag", "default", "child flag")
	rootCmd.AddCommand(childCmd)

	util.BindFlagsToViper(rootCmd, v)
	v.Set("child_flag", "bound-value")

	req.Equal("bound-value", v.GetString("child_flag"))
}

// --- InitializeConfiguration tests ---

func TestInitializeConfiguration_NoConfigFile(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()

	rootCmd := &cobra.Command{Use: "testapp"}
	util.AddConfigFlag(rootCmd)

	err := util.InitializeConfiguration(rootCmd, v)
	req.NoError(err)
}

func TestInitializeConfiguration_WithConfigFileFlag(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	v := viper.New()

	rootCmd := &cobra.Command{Use: "testapp"}
	util.AddConfigFlag(rootCmd)

	// Set config flag to a non-existent file - should not error since ReadInConfig is best-effort
	err := rootCmd.PersistentFlags().Set(util.ConfigFlag, "/tmp/nonexistent-config-file-karmafun.yaml")
	req.NoError(err)

	err = util.InitializeConfiguration(rootCmd, v)
	req.NoError(err)
}

func TestInitializeConfiguration_WithEnvVars(t *testing.T) {
	t.Setenv("TESTAPP_MY_FLAG", "env-value")
	req := require.New(t)
	v := viper.New()

	rootCmd := &cobra.Command{Use: "testapp"}
	util.AddConfigFlag(rootCmd)
	rootCmd.Flags().String("my-flag", "default", "test flag")

	err := util.InitializeConfiguration(rootCmd, v)
	req.NoError(err)
}

func TestInitializeConfiguration_LoadsConfigFileFromXDGConfigHome(t *testing.T) {
	req := require.New(t)
	v := viper.New()

	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	req.NoError(os.WriteFile(configDir+"/.testapp.yaml", []byte("my_flag: from-config\n"), 0o600))

	rootCmd := &cobra.Command{Use: "testapp"}
	util.AddConfigFlag(rootCmd)
	rootCmd.Flags().String("my-flag", "default", "test flag")

	err := util.InitializeConfiguration(rootCmd, v)
	req.NoError(err)
	req.Equal("from-config", rootCmd.Flags().Lookup("my-flag").Value.String())
}
