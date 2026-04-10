package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lutefd/weaver/internal/config"
	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	exitCodeOK     = 0
	exitCodeError  = 1
	exitCodeUsage  = 2
	exitCodeConfig = 3
)

type cliOptions struct {
	ConfigPath string
	Verbose    bool
}

type App struct {
	Options   cliOptions
	Runner    gitrunner.Runner
	Config    *config.Config
	ConfigErr error
}

var (
	opts    cliOptions
	rootCmd = &cobra.Command{
		Use:           "weaver",
		Short:         "Manage local branch stacks without hiding Git",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return bootstrapApp(cmd.Context(), cmd)
		},
	}
	app *App
)

func Execute() error {
	return rootCmd.Execute()
}

func ExitCode(err error) int {
	if err == nil {
		return exitCodeOK
	}

	var usageErr usageError
	if errors.As(err, &usageErr) {
		return exitCodeUsage
	}

	var cfgErr config.Error
	if errors.As(err, &cfgErr) {
		return exitCodeConfig
	}

	return exitCodeError
}

func AppContext() *App {
	return app
}

func bootstrapApp(ctx context.Context, cmd *cobra.Command) error {
	if ctx == nil {
		ctx = context.Background()
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	repoRoot, err := gitrunner.DiscoverRepoRoot(ctx, cwd)
	if err != nil {
		return fmt.Errorf("discover repository root: %w", err)
	}

	var trace io.Writer
	if opts.Verbose && cmd != nil {
		trace = cmd.OutOrStdout()
	}
	runner := gitrunner.NewCLIRunner(repoRoot, trace)
	cfg, err := loadConfig(repoRoot)
	var cfgErr error
	if err != nil {
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			if cmd == nil || cmd.Name() != "doctor" {
				return err
			}
			cfgErr = err
			defaultCfg := config.Default()
			cfg = &defaultCfg
		}
	}
	if cfg == nil {
		defaultCfg := config.Default()
		cfg = &defaultCfg
	}

	app = &App{
		Options:   opts,
		Runner:    runner,
		Config:    cfg,
		ConfigErr: cfgErr,
	}

	return nil
}

func loadConfig(repoRoot string) (*config.Config, error) {
	v := viper.New()
	path := opts.ConfigPath
	if path == "" {
		path = filepath.Join(repoRoot, config.FileName)
	}

	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	cfg := config.Default()
	if err := config.LoadInto(v, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &cfg, nil
		}
		return nil, err
	}

	return &cfg, nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "path to a config file")
	rootCmd.PersistentFlags().BoolVarP(&opts.Verbose, "verbose", "v", false, "enable verbose output")
}

type usageError struct {
	err error
}

func (e usageError) Error() string {
	return e.err.Error()
}

func markUsage(err error) error {
	if err == nil {
		return nil
	}
	return usageError{err: err}
}

func currentBranchName(ctx context.Context) (string, error) {
	return currentBranchNameForRunner(ctx, AppContext().Runner)
}

func currentBranchNameForRunner(ctx context.Context, runner gitrunner.Runner) (string, error) {
	result, err := runner.Run(ctx, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("resolve current branch: %w", err)
	}
	if result.Stdout == "" {
		return "", fmt.Errorf("resolve current branch: empty branch name")
	}
	return result.Stdout, nil
}
