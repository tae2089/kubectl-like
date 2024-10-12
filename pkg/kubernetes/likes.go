package kubernetes

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/logs"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
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

	l.hiddenGlobalFlags(cmd)

	filters := []string{"options"}
	templates.ActsAsRootCommand(cmd, filters)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
}

func (l *LikeOptions) hiddenGlobalFlags(cmd *cobra.Command) {
	hiddentFlags := []string{"namespace", "server", "kubeconfig", "context", "cluster", "as", "as-group", "as-uid", "cache-dir", "certificate-authority", "client-certificate", "client-key", "disable-compression", "insecure-skip-tls-verify", "request-timeout", "tls-server-name", "token", "user"}
	for _, flag := range hiddentFlags {
		cmd.Flags().MarkHidden(flag)
	}
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
