package kubernetes

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/logs"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/completion"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
)

const (
	logsUsageStr          = "like [-f] [-p] (POD | TYPE/NAME) [-c CONTAINER]"
	defaultPodLogsTimeout = 20 * time.Second
)

var (
	selectorTail    int64 = 10
	logsUsageErrStr       = fmt.Sprintf("expected '%s'.\nPOD or TYPE/NAME is a required argument for the logs command", logsUsageStr)
)

type LikeOptions struct {
	Pattern string
	*logs.LogsOptions
	KubernetesConfigFlags          *genericclioptions.ConfigFlags
	factory                        cmdutil.Factory
	containerNameFromRefSpecRegexp *regexp.Regexp
}

// NewLikeOptions creates a new LikeOptions struct
func NewLikeOptions(streams genericiooptions.IOStreams) LikeOptions {
	l := logs.NewLogsOptions(streams)
	KubernetesConfigFlags := genericclioptions.NewConfigFlags(true)
	f := cmdutil.NewFactory(KubernetesConfigFlags)

	return LikeOptions{
		KubernetesConfigFlags:          KubernetesConfigFlags,
		factory:                        f,
		LogsOptions:                    l,
		containerNameFromRefSpecRegexp: regexp.MustCompile(`spec\.(?:initContainers|containers|ephemeralContainers){(.+)}`),
	}
}

// AddFlags adds flags to the LikeOptions struct
func (l *LikeOptions) AddFlags(cmd *cobra.Command) {
	// Add flags from logs command
	l.LogsOptions.AddFlags(cmd)
	// Add flags from like command
	cmd.Flags().StringVar(&l.Pattern, "pattern", "*", "pattern to match logs with regex")
	// Add flags from kubectl command
	l.KubernetesConfigFlags.AddFlags(cmd.Flags())
	// reset help flag that is the help for kubectl and remove it from the command
	cmd.PersistentFlags().BoolP("help", "", false, "")
	cmd.PersistentFlags().MarkHidden("help")
}

// Complete fills in the gaps in the LikeOptions struct
func (l *LikeOptions) Complete(args []string, cmd *cobra.Command) error {
	if err := l.LogsOptions.Complete(l.factory, cmd, args); err != nil {
		return err
	}
	// Set the consume request function if the pattern is not empty
	// This is to ensure that the logs are filtered based on the pattern
	if l.Pattern != "" {
		l.LogsOptions.ConsumeRequestFn = l.DefaultConsumeRequest
	}
	return nil
}

// Validate ensures that all required arguments and flag values are provided
func (l LikeOptions) Vaildate() error {
	return l.LogsOptions.Validate()
}

// Run executes the LikeOptions
func (l LikeOptions) Run() error {
	return l.LogsOptions.RunLogs()
}

// DefaultConsumeRequest consumes the logs from the request and writes to the output
func (l LikeOptions) DefaultConsumeRequest(request rest.ResponseWrapper, out io.Writer) error {
	readCloser, err := request.Stream(context.TODO())
	if err != nil {
		return err
	}
	defer readCloser.Close()
	// Compile the regular expression
	re, err := regexp.Compile(l.Pattern)
	if err != nil {
		return err
	}

	r := bufio.NewReader(readCloser)
	for {
		bytes, err := r.ReadBytes('\n')
		if re.Match(bytes) {
			if _, err := out.Write(bytes); err != nil {
				return err
			}
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}

// RegisterCompletionFunc registers the completion functions for the LikeOptions
func (l *LikeOptions) RegisterCompletionFunc(cmd *cobra.Command) {
	utilcomp.SetFactoryForCompletion(l.factory)
	cmd.ValidArgsFunction = completion.PodResourceNameAndContainerCompletionFunc(l.factory)
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"namespace",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(l.factory, "namespace", toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"context",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.ListContextsInConfig(toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cluster",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.ListClustersInConfig(toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc(
		"user",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.ListUsersInConfig(toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
}
