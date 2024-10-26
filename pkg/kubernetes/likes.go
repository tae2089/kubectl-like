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

func (l *LikeOptions) AddFlags(cmd *cobra.Command) {
	l.LogsOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&l.Pattern, "pattern", "", "If true, print the logs for the previous instance of the container in a pod if it exists.")
	l.KubernetesConfigFlags.AddFlags(cmd.Flags())
	filters := []string{"options"}
	ActsAsRootCommand(cmd, filters)
}

func (l *LikeOptions) Complete(args []string, cmd *cobra.Command) error {
	if err := l.LogsOptions.Complete(l.factory, cmd, args); err != nil {
		return err
	}
	if l.Pattern != "" {
		l.LogsOptions.ConsumeRequestFn = l.DefaultConsumeRequest
	}
	return nil
}

func (l LikeOptions) Vaildate() error {
	return l.LogsOptions.Validate()
}

func (l LikeOptions) Run() error {
	return l.LogsOptions.RunLogs()
}

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
